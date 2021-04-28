// Golang program to show how to
// use structs as map keys
package main
  
// importing required packages
import (
	"fmt"
	"encoding/json"
	"time"
	"net/http"
	"strconv"
	"strings"

	
)  

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


type Options struct {
	Cookies []Cookie 
	name string

}

// Time wraps time.Time overriddin the json marshal/unmarshal to pass
// timestamp as integer
type Time struct {
	Time time.Time
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
	
	fmt.Println(buf, "test")
	// Try to manually parse the data
	tt, err := ParseDateString(str)
	if err != nil {
		return err
	}
	fmt.Println(tt)
	t.Time = tt
	return nil
}


type TLS struct {
	Time Time `json:"time"`
}

// ParseDateString takes a string and passes it through Approxidate
// Parses into a time.Time
func ParseDateString(dt string) (time.Time, error) {
	
	const layout = "Mon, 02-Jan-2006 15:04:05 MST"
	fmt.Println(layout, dt, "end")
	return time.Parse(layout, dt)
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
			Name: "test1", 
			Value: "arr", 
			Expires: time.Now(),
		},
		Cookie {
			Name: "test2", 
			Value: "b-option", 
			Expires: time.Now(),
		},
	}



	var d Time
	jStr := `{"time":"Mon, 02-Jan-2006 15:04:05 MST"}`
	_ = json.Unmarshal([]byte(jStr), &d)

	fmt.Println(d.Time)
	
	fmt.Println(Options{Cookies: []Cookie{ 
		Cookie {
			Name: "test1", 
			Value: "arr", 
			Expires: time.Now(),
		},
		Cookie {
			Name: "test2", 
			Value: "b-option", 
			Expires: time.Now(),
		},
	},})
	var appointment []Cookie
	err := json.Unmarshal(jsonData, &appointment)
	if err != nil {
		fmt.Printf("Error: ", err)
	}
	fmt.Println(basenameOpts, "\n")
	fmt.Println(appointment[0].Time)


}