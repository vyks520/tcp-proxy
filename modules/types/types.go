package types

type TaskDataJson struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Method string `json:"method"`
	Token  string `json:"token"`
	TaskID string `json:"task_id"`
}
