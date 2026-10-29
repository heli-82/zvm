package progress

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Writer struct {
	label string
	total int64 // <=0 means unknown
	done  int64
	start time.Time
	last  time.Time
	tty   bool
}

func New(label string, total int64) *Writer {
	fi, err := os.Stderr.Stat()
	tty := err == nil && fi.Mode()&os.ModeCharDevice != 0
	return &Writer{label: label, total: total, start: time.Now(), tty: tty}
}

func (p *Writer) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.tty && time.Since(p.last) > 100*time.Millisecond {
		p.last = time.Now()
		p.Render()
	}
	return len(b), nil
}

func (p *Writer) Speed() int64 {
	el := max(time.Since(p.start), time.Millisecond)
	return int64(float64(p.done) / el.Seconds())
}

func (p *Writer) Render() {
	if p.total <= 0 {
		fmt.Fprintf(os.Stderr, "\r\033[K%s %s (%s/s)", p.label, HumanSize(p.done), HumanSize(p.Speed()))
		return
	}
	const width = 30
	frac := min(float64(p.done)/float64(p.total), 1)
	filled := int(frac * width)
	bar := strings.Repeat("=", filled) + strings.Repeat(" ", width-filled)
	fmt.Fprintf(os.Stderr, "\r\033[K%s [%s] %3.0f%% %s/%s (%s/s)",
		p.label, bar, frac*100, HumanSize(p.done), HumanSize(p.total), HumanSize(p.Speed()))
}

func (p *Writer) Finish(err error) {
	if p.tty {
		fmt.Fprint(os.Stderr, "\r\033[K")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s failed: %v\n", p.label, err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s done (%s in %.1fs)\n", p.label, HumanSize(p.done), time.Since(p.start).Seconds())
}

func HumanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGT"[exp])
}
