package sleep

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/corverroos/replay"
	"github.com/corverroos/replay/db"
	"github.com/corverroos/replay/internal"
	"github.com/luno/fate"
	"github.com/luno/jettison/log"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rpatterns"
	"github.com/pkg/errors"
	"time"
)

//go:generate protoc --go_out=plugins=grpc:. ./sleep.proto

func RegisterForTesting(ctx context.Context, cl replay.Client, cstore reflex.CursorStore, dbc *sql.DB) {
	pollPeriod = time.Millisecond*100
	shouldComplete = func(completeAt time.Time) bool {
		return true
	}
	Register(ctx, cl, cstore, dbc)
}

func Register(ctx context.Context, cl replay.Client, cstore reflex.CursorStore, dbc *sql.DB) {
	fn := func(ctx context.Context, f fate.Fate, e *reflex.Event) error {
		if !reflex.IsType(e.Type, db.ActivityRequest) {
			return nil
		}

		id, message, err := internal.ParseEvent(e)
		if err != nil {
			return err
		}

		if id.Activity != internal.SleepActivity {
			return nil
		}


		req := message.(*internal.SleepRequest)
		completeAt := time.Now().Add(time.Duration(req.Duration.Seconds) * time.Second)

		_, err = dbc.ExecContext(ctx, "insert into sleeps set foreign_id=?, "+
			"created_at=?, complete_at=?, completed=false, async_token=?", e.ForeignID, time.Now(), completeAt, req.AsyncToken)
		if _, ok := db.MaybeWrapErrDuplicate(err, "foreign_id"); ok {
			return nil
		} else if err != nil {
			return err
		}

		return nil
	}

	spec := reflex.NewSpec(cl.Stream, cstore, reflex.NewConsumer(internal.SleepActivity, fn))
	go rpatterns.RunForever(func() context.Context { return ctx }, spec)
	go completeSleepsForever(ctx, cl, dbc)
}

func completeSleepsForever(ctx context.Context, cl replay.Client, dbc *sql.DB) {
	for {
		err := completeSleepsOnce(ctx, cl, dbc)
		if err != nil {
			log.Error(ctx, errors.Wrap(err, "complete sleeps once"))
		}
		time.Sleep(pollPeriod)
	}
}

func completeSleepsOnce(ctx context.Context, cl replay.Client, dbc *sql.DB) error {
	sl, err := listToComplete(ctx, dbc)
	if err != nil {
		return errors.Wrap(err, "list to complete")
	}

	for _, s := range sl {
		if !shouldComplete(s.CompleteAt) {
			return nil
		}

		var id db.EventID
		if err := json.Unmarshal([]byte(s.ForeignID), &id); err != nil {
			return err
		}

		if s.AsyncToken != "" {
			err := cl.CompleteAsyncActivity(ctx, s.AsyncToken, &internal.SleepDone{})
			if err != nil {
				return err
			}
		} else {
			err := cl.CompleteActivity(ctx, id.Workflow, id.Run, id.Activity, id.Index, &internal.SleepDone{})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

var pollPeriod = time.Second
var shouldComplete = func(completeAt time.Time) bool {
	return completeAt.Before(time.Now())
}

type sleep struct {
	ID         int
	ForeignID  string
	CreatedAt  time.Time
	CompleteAt time.Time
	AsyncToken string
}

func listToComplete(ctx context.Context, dbc *sql.DB) ([]sleep, error) {
	rows, err := dbc.QueryContext(ctx, "select id, foreign_id, created_at, complete_at, async_token "+
		"from sleeps where completed=false order by complete_at asc")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []sleep
	for rows.Next() {
		var s sleep

		err := rows.Scan(&s.ID, &s.ForeignID, &s.CreatedAt, &s.CompleteAt, &s.AsyncToken)
		if err != nil {
			return nil, err
		}

		res = append(res, s)
	}

	return res, rows.Err()
}