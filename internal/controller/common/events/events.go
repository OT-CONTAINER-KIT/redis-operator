package events

const (
	EventReasonRedisClusterDownscale = "RedisClusterDownscale"
	// RedisClusterRejoinFailed reports a pod that fell out of the cluster and
	// could not be attached back to its shard automatically.
	RedisClusterRejoinFailed = "RedisClusterRejoinFailed"
)

type Event struct {
	EventType string
	Reason    string
	Message   string
}

type Recorder struct {
	events []Event
}

func NewRecorder() *Recorder {
	return &Recorder{events: []Event{}}
}

func (r *Recorder) AddEvent(typ, reason, message string) {
	if r.events == nil {
		r.events = []Event{}
	}
	r.events = append(r.events, Event{EventType: typ, Reason: reason, Message: message})
}

func (r *Recorder) Events() []Event {
	return r.events
}
