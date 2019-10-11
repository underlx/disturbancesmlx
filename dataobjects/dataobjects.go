package dataobjects

import (
	"database/sql/driver"
	"errors"
	"time"

	msgpack "gopkg.in/vmihailenco/msgpack.v2"

	"fmt"

	"strings"

	sq "github.com/Masterminds/squirrel"
)

var sdb sq.StatementBuilderType

func init() {
	sdb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}

// Point represents bidimentional coordinates (such as GPS coordinates as used by Google Maps)
type Point [2]float64

// Value implements the driver.Value interface
func (p Point) Value() (driver.Value, error) {
	return fmt.Sprintf("(%f,%f)", p[0], p[1]), nil
}

// Scan implements the sql.Scanner interface
func (p *Point) Scan(val interface{}) error {
	b, ok := val.([]byte)
	if !ok {
		return errors.New("Scan: Invalid val type for scanning")
	}
	_, err := fmt.Sscanf(string(b), "(%f,%f)", &p[0], &p[1])
	return err
}

// Time wraps a time.Time with custom methods for serialization
type Time time.Time

var _ msgpack.CustomEncoder = (*Time)(nil)
var _ msgpack.CustomDecoder = (*Time)(nil)

var timeLayout = "15:04:05"

// ErrTimeParse is returned when time unmarshaling fails
var ErrTimeParse = errors.New(`TimeParseError: should be a string formatted as "15:04:05"`)

// MarshalJSON implements json.Marshaler
func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).Format(timeLayout) + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (t *Time) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) != len(timeLayout)+2 {
		return ErrTimeParse
	}
	ret, err := time.Parse(timeLayout, s[1:len(timeLayout)-1])
	if err != nil {
		return err
	}
	*t = Time(ret)
	return nil
}

// EncodeMsgpack implements the msgpack.CustomEncoder interface
func (t Time) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.Encode(int(time.Time(t).Sub(time.Time{}.AddDate(-1, 0, 0)).Seconds()))
}

// DecodeMsgpack implements the msgpack.CustomDecoder interface
func (t *Time) DecodeMsgpack(dec *msgpack.Decoder) error {
	var i int
	err := dec.Decode(&i)
	if err != nil {
		return err
	}
	*t = Time(time.Time{}.AddDate(-1, 0, 0).Add(time.Duration(i) * time.Second))
	return nil
}

// Scan implements the sql.Scanner interface.
func (t *Time) Scan(value interface{}) error {
	*t = Time(value.(time.Time))
	return nil
}

// Value implements the driver.Valuer interface.
func (t Time) Value() (driver.Value, error) {
	return time.Time(t), nil
}

// Duration wraps a time.Duration with custom methods for serialization
type Duration time.Duration

var _ msgpack.CustomEncoder = (*Duration)(nil)
var _ msgpack.CustomDecoder = (*Duration)(nil)

// MarshalJSON implements json.Marshaler
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Duration(d).String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (d *Duration) UnmarshalJSON(b []byte) error {
	duration, err := time.ParseDuration(strings.Trim(string(b), "\""))
	*d = Duration(duration)
	return err
}

// EncodeMsgpack implements the msgpack.CustomEncoder interface
func (d Duration) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.Encode(int(time.Duration(d).Seconds()))
}

// DecodeMsgpack implements the msgpack.CustomDecoder interface
func (d *Duration) DecodeMsgpack(dec *msgpack.Decoder) error {
	var i int
	err := dec.Decode(&i)
	if err != nil {
		return err
	}
	*d = Duration(time.Duration(i) * time.Second)
	return nil
}

// Scan implements the sql.Scanner interface.
func (d *Duration) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("Scan: Invalid val type for scanning")
	}
	ss := strings.Split(string(b), ":")
	var hour, minute, second int
	fmt.Sscanf(ss[0], "%d", &hour)
	fmt.Sscanf(ss[1], "%d", &minute)
	fmt.Sscanf(ss[2], "%d", &second)
	*d = Duration(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute + time.Duration(second)*time.Second)
	return nil
}

// Value implements the driver.Valuer interface.
func (d Duration) Value() (driver.Value, error) {
	return time.Duration(d).String(), nil
}

func getCacheKey(objtype string, other ...interface{}) string {
	elem := make([]string, len(other))
	for i, e := range other {
		elem[i] = fmt.Sprint(e)
	}
	return strings.Join(append([]string{"do", objtype}, elem...), "-")
}
