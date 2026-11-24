package llm

import (
	"bufio"
	"io"
	"strings"
)

// maxSSELine 是 SSE 单行的上限（E.10.01 §5.4）。
const maxSSELine = 4 << 20

// Event 是一个 SSE 事件。
type Event struct {
	Event string
	Data  string // 多行 data: 用 \n 连接
	ID    string
}

// SSEReader 是三种协议共用的 SSE 读取器（E.10.01 §5.4）：按行读，event:、data:、id:，
// 以 : 开头的注释行忽略，空行结束一个事件。data: [DONE] 由协议自己判断。
type SSEReader struct {
	sc *bufio.Scanner
}

// NewSSEReader 从 r 读 SSE。
func NewSSEReader(r io.Reader) *SSEReader {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxSSELine)
	return &SSEReader{sc: sc}
}

// Next 返回下一个事件；读完时返回 io.EOF。最后一个事件后面没有空行时也照常返回它。
func (s *SSEReader) Next() (Event, error) {
	var ev Event
	var data []string
	has := false
	for s.sc.Scan() {
		line := strings.TrimSuffix(s.sc.Text(), "\r")
		if line == "" {
			if has {
				ev.Data = strings.Join(data, "\n")
				return ev, nil
			}
			ev = Event{}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			ev.Event = value
		case "data":
			data = append(data, value)
			has = true
		case "id":
			ev.ID = value
		}
	}
	if err := s.sc.Err(); err != nil {
		return Event{}, err
	}
	if has {
		ev.Data = strings.Join(data, "\n")
		return ev, nil
	}
	return Event{}, io.EOF
}
