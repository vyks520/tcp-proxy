package types

type TaskDataJson struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Method string `json:"method"`
	TaskID string `json:"task_id"`
}
