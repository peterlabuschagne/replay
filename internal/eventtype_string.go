// Code generated by "stringer -type=EventType"; DO NOT EDIT.

package internal

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[RunCreated-1]
	_ = x[RunCompleted-2]
	_ = x[RunOutput-3]
	_ = x[ActivityRequest-4]
	_ = x[ActivityResponse-5]
}

const _EventType_name = "RunCreatedRunCompletedRunOutputActivityRequestActivityResponse"

var _EventType_index = [...]uint8{0, 10, 22, 31, 46, 62}

func (i EventType) String() string {
	i -= 1
	if i < 0 || i >= EventType(len(_EventType_index)-1) {
		return "EventType(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _EventType_name[_EventType_index[i]:_EventType_index[i+1]]
}
