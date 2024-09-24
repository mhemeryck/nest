package main

import (
	"log"
	"time"

	"github.com/mhemeryck/nest/pkg/device"
)

var (
	// direct mapping between input and output
	automations = map[string]string{
		"do-1-01": "ro-2-13",
	}

	pushButtonToLight = map[EntityId]EntityId{
		"office": "office-desk",
	}
)

type EntityId string

type Event struct {
	// TODO: dedicated type?
	EntityId EntityId
	// TODO: enum-like message?
	Message string
}

// Hive is the hive mind that knows all
type Hive struct {
	EventC chan Event

	pushButtonToLightMap map[EntityId]EntityId
	PushButtons          map[EntityId]*PushButton
	Lights               map[EntityId]*Light
}

func (h Hive) Loop() {
	for msg := range h.EventC {
		log.Printf("received msg %v", msg)
		if id, ok := h.pushButtonToLightMap[msg.EntityId]; ok {
			log.Printf("Did get a match: %s -> %s!", msg.EntityId, id)
			if l, ok := h.Lights[id]; ok {
				l.Toggle()
				log.Printf("Here's the light %s", l)
			}
		}
	}
}

type Entity struct {
	Id     EntityId
	EventC chan Event
}

type PushButton struct {
	Id     EntityId
	EventC chan Event
	// TODO: enum-like; with 3-state?
	State bool
}

func (p *PushButton) Toggle() {
	// Change current state
	p.State = !p.State

	var m string
	switch p.State {
	case false:
		m = "off"
	case true:
		m = "on"
	}
	p.EventC <- Event{
		EntityId: p.Id,
		Message:  m,
	}
}

type Light struct {
	Entity

	// TODO: enum
	State bool
}

func (l *Light) Toggle() {
	l.State = !l.State

	// var m string
	// switch l.State {
	// case false:
	// 	m = "off"
	// case true:
	// 	m = "on"
	// }
	// l.EventC <- Event{
	// 	EntityId: l.Id,
	// 	Message:  m,
	// }
}

func main2() {
	reader := make(chan device.DevicePayload)
	mgr, err := device.NewDeviceManagerFromPath("test/fixtures", reader)
	if err != nil {
		log.Fatalf("Can't start a device manager: %v", err)
	}
	defer mgr.Close()

	for _, d := range mgr.Devices {
		log.Printf("Found device %v\n", d)
		go d.Loop()
	}

	go func() {
		for msg := range reader {
			log.Printf("Reader got value %v", msg)
			if output, ok := automations[msg.Id]; ok {
				log.Printf("Got a match on the input, can now trigger the output: %v", output)
				var payload bool
				if msg.Message == device.MessageType_TurnOff {
					payload = false
				} else if msg.Message == device.MessageType_TurnOn {
					payload = true
				}
				err = mgr.Devices[output].Write(payload)
				if err != nil {
					panic(err)
				}
			}
		}

	}()

	d := mgr.Devices["do-1-01"]
	log.Printf("device %v\n", d)
	for i := 0; i < 5; i++ {
		d.Write(i%2 == 0)
		time.Sleep(3 * time.Second)
	}
}

func main() {
	eventChan := make(chan Event)
	p := PushButton{
		Id:     "office",
		EventC: eventChan,
		State:  false,
	}

	l := Light{
		Entity: Entity{
			Id:     "office-desk",
			EventC: eventChan,
		},
		State: false,
	}

	hive := Hive{
		EventC:               eventChan,
		pushButtonToLightMap: pushButtonToLight,
		PushButtons: map[EntityId]*PushButton{
			p.Id: &p,
		},
		Lights: map[EntityId]*Light{
			l.Id: &l,
		},
	}

	go hive.Loop()

	for {
		p.Toggle()

		time.Sleep(time.Millisecond * 500)
	}
}
