package daemon

import (
	"bytes"
	"encoding/xml"
	"io"
	"sort"
	"strings"
)

type logEvent struct {
	ID      int64    `xml:"System>EventRecordID"`
	Data    []string `xml:"EventData>Data"`
	Message string   `xml:"RenderingInfo>Message"`
}

func parseWindowsEvents(data []byte, cursor int64) ([]string, int64, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var events []logEvent
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, cursor, err
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "Event" {
			var ev logEvent
			if err := decoder.DecodeElement(&ev, &start); err != nil {
				return nil, cursor, err
			}
			events = append(events, ev)
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].ID < events[j].ID })
	var lines []string
	for _, ev := range events {
		if ev.ID <= cursor {
			continue
		}
		cursor = ev.ID
		message := ev.Message
		if message == "" {
			message = strings.Join(ev.Data, " ")
		}
		lines = append(lines, message)
	}
	return lines, cursor, nil
}
