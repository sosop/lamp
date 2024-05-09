package utils

type RResult struct {
	Code int
	Msg  string
	Data []ConnData
}

type PResult struct {
	Code int
	Msg  string
}

type ConnData struct {
	DeviceCode  string
	BeatContent string
	BeatTime    int
}
