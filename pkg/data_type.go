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

type ReqMsgRemove struct{}
type ResMsgRemove struct{}

type ReqMsgStatus struct{}
type ResMsgStatus struct {
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
}

type ReqMsgLogs = struct{}
type ResMsgLogs = struct {
	Log string `json:"log"`
}

const (
	TAG_REQUEST_UPLOAD  uint32 = 0xE201
	TAG_REQUEST_CREATE  uint32 = 0xE202
	TAG_REQUEST_REMOVE  uint32 = 0xE205
	TAG_REQUEST_STATUS  uint32 = 0xE206
	TAG_REQUEST_LOGS    uint32 = 0xE207
	TAG_RESPONSE_UPLOAD uint32 = 0xF201
	TAG_RESPONSE_CREATE uint32 = 0xF202
	TAG_RESPONSE_REMOVE uint32 = 0xF205
	TAG_RESPONSE_STATUS uint32 = 0xF206
	TAG_RESPONSE_LOGS   uint32 = 0xF207
)

func ResponseTag(tag uint32) uint32 {
	return tag + 0x1000
}
