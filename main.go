/*
 * Copyright (c) Clinton Freeman 2018
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
 * associated documentation files (the "Software"), to deal in the Software without restriction,
 * including without limitation the rights to use, copy, modify, merge, publish, distribute,
 * sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all copies or
 * substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT
 * NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
 * NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
 * DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/hypebeast/go-osc/osc"
)

type Vote int

const (
	N Vote = iota
	A
	B
	C
)

var mutex = &sync.Mutex{}
var seats [12]Vote = [12]Vote{N, N, N, N, N, N, N, N, N, N, N, N}

func parse(address string, direction string) int {
	a := strings.Split(address, "/")
	n := strings.Split(a[2], direction)
	sensor, err := strconv.Atoi(n[0])
	if err != nil {
		fmt.Println("Can't parse address")
		return -1
	}

	return sensor
}

func notifyQlab(votes [12]Vote) {
	aTally := 0
	bTally := 0
	cTally := 0

	fmt.Print("Seats: ")
	for i := 0; i < 12; i++ {
		switch votes[i] {
		case A:
			fmt.Print("A, ")
			aTally = aTally + 1
		case B:
			fmt.Print("B, ")
			bTally = bTally + 1
		case C:
			fmt.Print("C, ")
			cTally = cTally + 1
		default:
			fmt.Print("N, ")
		}
	}
	fmt.Println("")

	client := osc.NewClient("localhost", 53000)

	msg := osc.NewMessage("/cue/aTally" + strconv.Itoa(aTally) + "/start")
	client.Send(msg)

	msg = osc.NewMessage("/cue/bTally" + strconv.Itoa(bTally) + "/start")
	client.Send(msg)

	msg = osc.NewMessage("/cue/cTally" + strconv.Itoa(cTally) + "/start")
	client.Send(msg)

	msg = osc.NewMessage("/cue/total" + strconv.Itoa(aTally+bTally+cTally) + "/start")
	client.Send(msg)

	fmt.Println("Current: [a + b + c ] = T", aTally, bTally, cTally, (aTally + bTally + cTally))

	if aTally > bTally && aTally > cTally {
		msg = osc.NewMessage("/cue/aWin/start")
		client.Send(msg)
	} else if bTally > aTally && bTally > cTally {
		msg = osc.NewMessage("/cue/bWin/start")
		client.Send(msg)
	} else if cTally > aTally && cTally > bTally {
		msg = osc.NewMessage("/cue/cWin/start")
		client.Send(msg)
	} else if aTally == bTally && aTally != cTally {
		msg = osc.NewMessage("/cue/abTie/start")
		client.Send(msg)
	} else if aTally == cTally && aTally != bTally {
		msg = osc.NewMessage("/cue/acTie/start")
		client.Send(msg)
	} else if cTally == bTally && aTally != bTally {
		msg = osc.NewMessage("/cue/cbTie/start")
		client.Send(msg)
	} else {
		msg = osc.NewMessage("/cue/abcTie/start")
		client.Send(msg)
	}
}

func ackMessage(msg *osc.Message) {
	// Let the sensor know that we got the message.
	from := strings.Split(msg.Address, ":")
	port, err := strconv.Atoi(from[1])
	if err != nil {
		port = 53001
	}

	client := osc.NewClient(from[0], port)
	client.Send(osc.NewMessage("/ack"))

	// Forward the message we recieved to the Qlab
	client = osc.NewClient("localhost", 53000)
	client.Send(msg)
}

func incTally(msg *osc.Message) {
	ackMessage(msg)

	sensor := parse(msg.Address, "on")
	fmt.Println("Received ON from: ", msg.Address)

	mutex.Lock()

	seat := (int)(math.Floor((float64)((sensor - 1) / 3.0)))
	fmt.Println("Seat: ", seat)
	fmt.Println("Seat Status: ", seats[seat])

	switch sensor % 3 {
	case 1:
		seats[seat] = A
		break

	case 2:
		seats[seat] = B
		break

	case 0:
		seats[seat] = C
		break
	}

	notifyQlab(seats)
	mutex.Unlock()
}

func decTally(msg *osc.Message) {
	ackMessage(msg)

	sensor := parse(msg.Address, "off")
	fmt.Println("Received OFF from: ", msg.Address)

	mutex.Lock()
	defer mutex.Unlock()

	// Ignore lifting from a tag.
	seat := (int)(math.Floor((float64)((sensor - 1) / 3.0)))
	fmt.Println("Seat: ", seat)
	fmt.Println("Seat Status: ", seats[seat])

	seats[seat] = N

	notifyQlab(seats)
}

func reset(msg *osc.Message) {
	fmt.Println("Resetting Tally")
	mutex.Lock()
	seats = [12]Vote{N, N, N, N, N, N, N, N, N, N, N, N}
	mutex.Unlock()
}

func main() {
	fmt.Println("Starting Tally v14")

	addr := "10.0.1.2:8765"
	d := osc.NewStandardDispatcher()

	for i := 1; i <= 36; i++ {
		addr := "/cue/" + strconv.Itoa(i) + "on/start"
		d.AddMsgHandler(addr, incTally)
	}

	for i := 1; i <= 36; i++ {
		addr := "/cue/" + strconv.Itoa(i) + "off/start"
		d.AddMsgHandler(addr, decTally)
	}

	d.AddMsgHandler("/reset", reset)

	server := &osc.Server{
		Addr:       addr,
		Dispatcher: d,
	}
	server.ListenAndServe()
}
