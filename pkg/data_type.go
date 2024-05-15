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
	TAG_REQUEST_UPLOAD  uint32 = 0xE201
	TAG_REQUEST_CREATE  uint32 = 0xE202
	TAG_REQUEST_STOP    uint32 = 0xE203
	TAG_REQUEST_START   uint32 = 0xE204
	TAG_REQUEST_REMOVE  uint32 = 0xE205
	TAG_REQUEST_STATUS  uint32 = 0xE206
	TAG_REQUEST_LOGS    uint32 = 0xE207
	TAG_RESPONSE_UPLOAD uint32 = 0xF201
	TAG_RESPONSE_CREATE uint32 = 0xF202
	TAG_RESPONSE_STOP   uint32 = 0xF203
	TAG_RESPONSE_START  uint32 = 0xF204
	TAG_RESPONSE_REMOVE uint32 = 0xF205
	TAG_RESPONSE_STATUS uint32 = 0xF206
	TAG_RESPONSE_LOGS   uint32 = 0xF207
)

func ResponseTag(tag uint32) uint32 {
	return tag + 0x1000
}
