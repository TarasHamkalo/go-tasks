package downloader

type EventType string

const (
	TaskError EventType = "TaskError"

	TaskStarted EventType = "TaskStarted"

	TaskData EventType = "TaskData"

	TaskDone EventType = "TaskDone"
)

type Event interface {
	TaskId() string

	EventType() EventType

	Properties() map[string]any
}

type BaseEvent struct {
	taskId     string
	eventType  EventType
	properties map[string]any
}

func NewBaseEvent(
	taskId string,
	eventType EventType,
	properties map[string]any,
) *BaseEvent {
	return &BaseEvent{
		taskId:     taskId,
		eventType:  eventType,
		properties: properties,
	}
}

func (b BaseEvent) TaskId() string {
	return b.taskId
}

func (b BaseEvent) Properties() map[string]any {
	return b.properties
}

func (b BaseEvent) EventType() EventType {
	return b.eventType
}

func NewStartedEvent(taskId string) *BaseEvent {
	return NewBaseEvent(taskId, TaskStarted, make(map[string]any))
}

func NewDoneEvent(taskId string) *BaseEvent {
	return NewBaseEvent(taskId, TaskDone, make(map[string]any))
}

func NewDataEvent(taskId string, data any) *BaseEvent {
	return NewBaseEvent(taskId, TaskData, map[string]any{"data": data})
}

func NewErrorEvent(taskId string, err error) *BaseEvent {
	return NewBaseEvent(taskId, TaskError, map[string]any{"error": err})
}
