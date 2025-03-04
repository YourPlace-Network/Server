package core

// this contains implementations of our local job queue'ing system

var channel = make(chan string, 300)

func QueueAdd(message string) {
	channel <- message
}

func QueuePop() (message string) {
	return <-channel
}
