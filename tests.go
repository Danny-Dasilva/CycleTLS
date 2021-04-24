// Golang program to show how to
// use structs as map keys
package main
  
// importing required packages
import (
	"fmt"
	"encoding/json"
)  
//declaring a struct
type Address struct {
    Name    string
    city    string
    Pincode int
}

  
// Contains everything about an appointment
type Appointment struct {
    Date    string `json:"Date"`    // Contains date as string
    StartMn string `json:"StartMn"` // Our startMn ?
    ID      int    `json:"ID"`      // AppointmentId
    UserID  int    `json:"UserID"`  // UserId
}

func main() {
    
	jsonData := []byte(`[
	{
		"Date": "Standard",
		"StartMn": "aaaaaaa",
		"ID": 999,
		"UserID": 3
	}]`)
	var appointment []Appointment
	err := json.Unmarshal(jsonData, &appointment)
	if err != nil {
		fmt.Printf("Error: ", err)
	}
	fmt.Println("Error: ", appointment)

}