package pkg

import "time"

type Request[T any] struct {
	Version uint32 `json:"version"`
	Target  string `json:"target"`
	SfnName string `json:"sfn_name"`
	Msg     *T     `json:"msg"`
}

type Response struct {
	MeshZone string `json:"mesh_zone"`
	MeshNode string `json:"mesh_node"`
	Done     bool   `json:"done"`
	Error    string `json:"error"`
	Msg      string `json:"msg"`
}

type ReqMsgUpload struct {
	ZipData []byte `json:"zip_data"`
}
type ResMsgUpload struct {
	Log string `json:"log"`
}

type ReqMsgCreate struct {
	Envs *[]string `json:"envs"`
}
type ResMsgCreate struct{}

type ReqMsgStop struct {
	Timeout *int `json:"timeout"`
}
type ResMsgStop struct{}

type ReqMsgStart struct{}
type ResMsgStart struct{}

type ReqMsgRemove struct{}
type ResMsgRemove struct{}

type ReqMsgStatus struct{}
type ResMsgStatus struct {
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

type ReqMsgLogs = struct {
	Tail *int `json:"tail"`
}
type ResMsgLogs = struct {
	Log string `json:"log"`
}

const (
	TAG_UPLOAD          uint32 = 0xFF01
	TAG_CREATE          uint32 = 0xFF02
	TAG_STOP            uint32 = 0xFF03
	TAG_START           uint32 = 0xFF04
	TAG_REMOVE          uint32 = 0xFF05
	TAG_STATUS          uint32 = 0xFF06
	TAG_LOGS            uint32 = 0xFF07
	TAG_RESPONSE_UPLOAD uint32 = 0xFF21
	TAG_RESPONSE_CREATE uint32 = 0xFF22
	TAG_RESPONSE_STOP   uint32 = 0xFF23
	TAG_RESPONSE_START  uint32 = 0xFF24
	TAG_RESPONSE_REMOVE uint32 = 0xFF25
	TAG_RESPONSE_STATUS uint32 = 0xFF26
	TAG_RESPONSE_LOGS   uint32 = 0xFF27
)

func TagName(tag uint32) string {
	switch tag {
	case TAG_UPLOAD:
		return "upload"
	case TAG_CREATE:
		return "create"
	case TAG_START:
		return "start"
	case TAG_STOP:
		return "stop"
	case TAG_REMOVE:
		return "remove"
	case TAG_STATUS:
		return "status"
	case TAG_LOGS:
		return "logs"
	default:
		return ""
	}
}

func ResponseTag(tag uint32) uint32 {
	return tag + 0x20
}
