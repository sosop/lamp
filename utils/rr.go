package utils


type RResult struct {
	Code	int
	Msg		string
	Data	[]ConnData
}

type ConnData struct {
	DeviceCode	string
	BeatContent	string
	BeatTime	int
}