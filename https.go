// Golang program to show how to
// use structs as map keys
package main
  
// importing required packages
import (
	"fmt"
	"encoding/json"
	"time"
	"net/http"
	"errors"
	"strconv"
	"strings"

	
)  
//declaring a struct
type Address struct {
    Name    string
    city    string
    Pincode int
}
// A Cookie represents an HTTP cookie as sent in the Set-Cookie header of an
// HTTP response or the Cookie header of an HTTP request.
//
// See https://tools.ietf.org/html/rfc6265 for details.
//Stolen from Net/http/cookies 
type Cookie struct {
	Name  string           `json:"name"` 
	Value string		   `json:"value"` 

	Path       string      `json:"path"` // optional
	Domain     string      `json:"domain"` // optional
	Expires    time.Time   `json:"expires"` // optional
	RawExpires string      `json:"rawExpires"`// for reading cookies only

	// MaxAge=0 means no 'Max-Age' attribute specified.
	// MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0'
	// MaxAge>0 means Max-Age attribute present and given in seconds
	MaxAge   int           `json:"maxAge"`
	Secure   bool          `json:"secure"`
	HttpOnly bool          `json:"httpOnly"`
	SameSite http.SameSite `json:"sameSite"`
	Raw      string
	Unparsed []string      `json:"unparsed"` // Raw text of unparsed attribute-value pairs
	Time Time `json:"time"`
}

// var Cookies = []Cookie 
// { 
//     Cookie {
//         shortnm: 'a', 
//         longnm: "multiple", 
//         needArg: false, 
//         help: "Usage for a",
//     },
//     Cookie {
//         shortnm: 'b', 
//         longnm: "b-option", 
//         needArg: false, 
//         help: "Usage for b",
//     },
// }
  
// Contains everything about an appointment

// {
// 	"Name": "Standard",
// 	"value": 999,
// 	"Domain": [
// 		"Apple",
// 		"Banana",
// 		"Orange"
// 	],
	
// 	"UserID": "2018-04-09T23:00:00Z"
// }`)





func test(t []Cookie)  (r []Cookie){
	t = r 
	return
}
type Options struct {
	Cookies []Cookie 
	name string

}


























// Format enum type.
type Format int32

// Format enum values.
const (
	Timestamp Format = iota
	TimestampNano
	ANSIC
	UnixDate
	RubyDate
	RFC822
	RFC822Z
	RFC850
	RFC1123
	RFC1123Z
	RFC3339
	RFC3339Nano
	Kitchen
)

// Common errors.
var (
	ErrInvalidFormat = errors.New("invalid format")
)

// Time wraps time.Time overriddin the json marshal/unmarshal to pass
// timestamp as integer
type Time struct {
	time.Time `bson:",inline"`
	format    Format
}

func (t Time) formatTime(mode int) ([]byte, error) {
	var ret string

	switch t.format {
	case ANSIC:
		ret = t.Time.Format(time.ANSIC)
	case UnixDate:
		ret = t.Time.Format(time.UnixDate)
	case RubyDate:
		ret = t.Time.Format(time.RubyDate)
	case RFC822:
		ret = t.Time.Format(time.RFC822)
	case RFC822Z:
		ret = t.Time.Format(time.RFC822Z)
	case RFC850:
		ret = t.Time.Format(time.RFC850)
	case RFC1123:
		ret = t.Time.Format(time.RFC1123)
	case RFC1123Z:
		ret = t.Time.Format(time.RFC1123Z)
	case RFC3339:
		ret = t.Time.Format(time.RFC3339)
	case RFC3339Nano:
		ret = t.Time.Format(time.RFC3339Nano)
	case Kitchen:
		ret = t.Time.Format(time.Kitchen)
	case Timestamp:
		return []byte(strconv.FormatInt(t.Time.Unix(), 10)), nil
	case TimestampNano:
		return []byte(strconv.FormatInt(t.Time.UnixNano(), 10)), nil
	default:
		return nil, ErrInvalidFormat
	}
	switch mode {
	default:
		fallthrough
	case 0: // json
		return []byte(`"` + ret + `"`), nil
	case 1: // bson
		return []byte(ret), nil
	}
}

// MarshalJSON implements json.Marshaler interface.
func (t Time) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	return t.formatTime(0)
}

// UnmarshalJSON implements json.Unmarshaler inferface.
func (t *Time) UnmarshalJSON(buf []byte) error {
	// Try to parse the timestamp integer
	ts, err := strconv.ParseInt(string(buf), 10, 64)
	if err == nil {
		if len(buf) == 19 {
			t.Time = time.Unix(ts/1e9, ts%1e9)
		} else {
			t.Time = time.Unix(ts, 0)
		}
		return nil
	}
	// Try the default unmarshal
	if err := json.Unmarshal(buf, &t.Time); err == nil {
		return nil
	}
	str := strings.Trim(string(buf), `"`)
	if str == "null" || str == "" {
		return nil
	}
	// Try to manually parse the data
	tt, err := ParseDateString(str)
	if err != nil {
		return err
	}
	t.Time = tt
	return nil
}



// ParseDateString takes a string and passes it through Approxidate
// Parses into a time.Time
func ParseDateString(dt string) (time.Time, error) {
	
	const layout = "Mon, 02-Jan-2006 15:04:05 MST"
  
	return time.Parse(layout, dt)
}










































type data struct {
	Time Time `json:"time"`
}

func main() {
    

	jsonData := []byte(`[
	{
		"name": "Standard",
		"value": "aaaaaaa",
		"time" : "Mon, 02-Jan-2006 15:04:05 MST"
	}]`)


	var basenameOpts = []Cookie{ 
		Cookie {
			Name: "a", 
			Value: "multiple", 
			Expires: time.Now(),
		},
		Cookie {
			Name: "yaaah", 
			Value: "b-option", 
			Expires: time.Now(),
		},
	}



	var d data
	jStr := `{"time":"Mon, 02-Jan-2006 15:04:05 MST"}`
	_ = json.Unmarshal([]byte(jStr), &d)

	fmt.Println(d.Time)
	
	fmt.Println(Options{Cookies: basenameOpts, name: "test"})
	var appointment []Cookie
	err := json.Unmarshal(jsonData, &appointment)
	if err != nil {
		fmt.Printf("Error: ", err)
	}
	fmt.Println(basenameOpts, "\n")
	fmt.Println(appointment[0].Time)


}