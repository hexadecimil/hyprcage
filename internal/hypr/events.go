package hypr

import (
	"bufio"
	"context"
	"net"
	"strings"
)

// Event is one line of Hyprland's event socket: NAME>>DATA.
type Event struct {
	Name string
	Data string
}

// Subscribe streams events from .socket2.sock until ctx is done or the
// connection drops. The handler runs on the calling goroutine.
func (i *Instance) Subscribe(ctx context.Context, handler func(Event)) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", i.socket(".socket2.sock"))
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		name, data, _ := strings.Cut(sc.Text(), ">>")
		handler(Event{Name: name, Data: data})
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return sc.Err()
}
