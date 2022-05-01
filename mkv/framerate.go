package mkv

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ztrue/tracerr"
)

type Framerate struct {
	Numerator   int
	Denominator int
}

func (framerate *Framerate) Parse(raw string) error {
	if strings.Contains(raw, "/") {
		split := strings.Split(raw, "/")

		num, err := strconv.ParseInt(split[0], 10, 32)
		if err != nil {
			return tracerr.Wrap(err)
		}

		denom, err := strconv.ParseInt(split[1], 10, 32)
		if err != nil {
			return tracerr.Wrap(err)
		}

		framerate.Numerator = int(num)
		framerate.Denominator = int(denom)
	} else {
		num, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return tracerr.Wrap(err)
		}

		framerate.Numerator = int(num)
		framerate.Denominator = 1
	}

	return nil
}

func (framerate Framerate) String() string {
	return fmt.Sprintf("%d/%d", framerate.Numerator, framerate.Denominator)
}

func (framerate Framerate) Value() float64 {
	return float64(framerate.Numerator) / float64(framerate.Denominator)
}
