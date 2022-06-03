package main

import (
   "fmt"
   "github.com/Danny-Dasilva/CycleTLS/cycletls"
)

const hello = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513-21,29-23-24,0"

func main() {
   opt := cycletls.Options{Ja3: hello}
   res, err := cycletls.Init().Do(
      "https://android.googleapis.com/auth", opt, "GET",
   )
   if err != nil {
      fmt.Println(err)
   }
   fmt.Println(res.Status)
}