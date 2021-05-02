package example

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luno/fate"
	"github.com/luno/jettison/jtest"
	"github.com/luno/jettison/log"
	"github.com/luno/reflex"
	"github.com/stretchr/testify/require"

	"github.com/corverroos/replay"
	"github.com/corverroos/replay/internal"
	"github.com/corverroos/replay/server"
	"github.com/corverroos/replay/test"
)

//go:generate protoc --go_out=plugins=grpc:. ./example.proto

func TestExample(t *testing.T) {
	_, cl, _, cstore := setup(t)

	var b Backends
	replay.RegisterActivity(oneCtx(t), cl, cstore, b, "ns", EnrichGreeting)
	replay.RegisterActivity(oneCtx(t), cl, cstore, b, "ns", PrintGreeting)
	replay.RegisterWorkflow(oneCtx(t), cl, cstore, "ns", GreetingWorkflow)

	ok, err := cl.RunWorkflow(context.Background(), "ns", "GreetingWorkflow", "run", &String{Value: "World"})
	jtest.RequireNil(t, err)
	require.True(t, ok)

	<-cl.completeChan
}

func TestExampleSleep(t *testing.T) {
	ctx, cl, dbc, cstore := setup(t)

	b := Backends{Replay: cl}
	test.RegisterNoopSleeps(ctx, cl, cstore, dbc)
	replay.RegisterActivity(oneCtx(t), cl, cstore, b, "ns", PrintGreeting)
	replay.RegisterWorkflow(oneCtx(t), cl, cstore, "ns", SleepWorkflow)

	ok, err := cl.RunWorkflow(ctx, "ns", "SleepWorkflow", "run", new(Empty))
	jtest.RequireNil(t, err)
	require.True(t, ok)

	<-cl.completeChan
}

func TestExampleSignal(t *testing.T) {
	ctx, cl, dbc, cstore := setup(t)

	b := Backends{Replay: cl}

	test.RegisterNoSleepSignals(ctx, cl.cl.(*server.DBClient), cstore, dbc)
	replay.RegisterActivity(oneCtx(t), cl, cstore, b, "ns", MaybeSignal)
	replay.RegisterActivity(oneCtx(t), cl, cstore, b, "ns", PrintGreeting)
	replay.RegisterWorkflow(oneCtx(t), cl, cstore, "ns", SignalWorkflow)

	ok, err := cl.RunWorkflow(ctx, "ns", "SignalWorkflow", "run", new(Empty))
	jtest.RequireNil(t, err)
	require.True(t, ok)

	<-cl.completeChan
}

func TestExampleGRPC(t *testing.T) {
	gcl, _ := test.SetupGRPC(t)
	cstore := new(test.MemCursorStore)
	cl := &testClient{
		Client:       gcl.Internal(),
		cl:           gcl,
		completeChan: make(chan string),
	}

	var b Backends

	replay.RegisterActivity(oneCtx(t), cl, cstore, b, "ns", EnrichGreeting)
	replay.RegisterActivity(oneCtx(t), cl, cstore, b, "ns", PrintGreeting)
	replay.RegisterWorkflow(oneCtx(t), cl, cstore, "ns", GreetingWorkflow)

	ok, err := cl.RunWorkflow(context.Background(), "ns", "GreetingWorkflow", "run", &String{Value: "World"})
	jtest.RequireNil(t, err)
	require.True(t, ok)

	<-cl.completeChan
}

type Backends struct {
	Replay replay.Client
}

func GreetingWorkflow(ctx replay.RunContext, name *String) {
	for i := 0; i < 5; i++ {
		name = ctx.ExecActivity(EnrichGreeting, name).(*String)
	}

	ctx.ExecActivity(PrintGreeting, name)
}

func SleepWorkflow(ctx replay.RunContext, _ *Empty) {
	for i := 0; i < 10; i++ {
		ctx.Sleep(time.Hour * 24 * 365)
	}

	ctx.ExecActivity(PrintGreeting, &String{Value: "sleepy head"})
}

func SignalWorkflow(ctx replay.RunContext, _ *Empty) {
	var sum int
	for i := 0; i < 10; i++ {
		ctx.ExecActivity(MaybeSignal, &Int{Value: int64(i)})
		res, ok := ctx.AwaitSignal(testsig{}, time.Second)
		if ok {
			sum += int(res.(*Int).Value)
		}
	}

	ctx.ExecActivity(PrintGreeting, &String{Value: fmt.Sprintf("sum %d", sum)})
}

func EnrichGreeting(ctx context.Context, b Backends, f fate.Fate, msg *String) (*String, error) {
	return &String{Value: "[" + msg.Value + "]"}, nil
}

func MaybeSignal(ctx context.Context, b Backends, f fate.Fate, i *Int) (*Empty, error) {
	if i.Value > 3 {
		return &Empty{}, nil
	}
	i.Value += 100

	_, err := b.Replay.SignalRun(ctx, "ns", "SignalWorkflow", "ns", testsig{}, i, fmt.Sprint(i.Value))
	return &Empty{}, err
}

func PrintGreeting(ctx context.Context, b Backends, f fate.Fate, msg *String) (*Empty, error) {
	log.Info(ctx, "Hello "+msg.Value)
	return &Empty{}, nil
}

type testsig struct{}

func (s testsig) SignalType() int {
	return 0
}

func (s testsig) MessageType() proto.Message {
	return &Int{}
}

// testClient wraps a replay client and add two features, completeChan that is notified when a run completes and
// blockActivity that will block runs from doing a specific activity request, blockchan is notified on each block.
type testClient struct {
	internal.Client
	cl            replay.Client
	completeChan  chan string
	blockedChan   chan string
	blockActivity map[string]error
}

// Clone returns a new test client with the same underlying replay client but with new empty test config.
func (c *testClient) Clone() *testClient {
	return &testClient{
		Client:        c.Client,
		cl:            c.cl,
		completeChan:  make(chan string),
		blockedChan:   make(chan string),
		blockActivity: make(map[string]error),
	}
}

func (c *testClient) RunWorkflow(ctx context.Context, namespace, workflow, run string, message proto.Message) (bool, error) {
	return c.cl.RunWorkflow(ctx, namespace, workflow, run, message)
}

func (c *testClient) SignalRun(ctx context.Context, namespace, workflow, run string, s replay.Signal, message proto.Message, extID string) (bool, error) {
	return c.cl.SignalRun(ctx, namespace, workflow, run, s, message, extID)
}

func (c *testClient) Stream(namespace string) reflex.StreamFunc {
	return c.cl.Stream(namespace)
}

func (c *testClient) Internal() internal.Client {
	return c
}

func (c *testClient) CompleteRun(ctx context.Context, key string) error {

	defer func() { c.completeChan <- key }()

	return c.Client.CompleteRun(ctx, key)
}

func (c *testClient) RequestActivity(ctx context.Context, key string, args proto.Message) error {
	k, err := internal.DecodeKey(key)
	if err != nil {
		return err
	}

	if err, ok := c.blockActivity[k.Activity]; ok {
		defer func() {
			c.blockedChan <- k.Activity
		}()
		return err
	}

	return c.Client.RequestActivity(ctx, key, args)
}

// oneCtx returns a getCtx function that only returns a single context. It blocks on subsequent calls.
func oneCtx(t *testing.T) func() context.Context {
	var n int
	return func() context.Context {
		if n > 0 {
			time.Sleep(time.Hour)
		}
		n++
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(func() {
			cancel()
		})
		return ctx
	}
}
