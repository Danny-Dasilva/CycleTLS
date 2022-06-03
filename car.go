package main

import (
    "log"

    tls "github.com/Carcraftz/utls"
    "github.com/Carcraftz/cclient"
)

func main() {
    client, err := cclient.NewClient(tls.HelloChrome_Auto,"",true,6)
    if err != nil {
        log.Fatal(err)
    }

    resp, err := client.Get("https://www.coinbase.com/users/oauth_signup?client_id=2d06b9a69c15e183856ff52c250281f6d93f9abef819921eac0d8647bb2b61f9&meta%5Baccount%5D=all&redirect_uri=https%3A%2F%2Fpro.coinbase.com%2Foauth_redirect&response_type=code&scope=user+balance&state=")
    if err != nil {
        log.Fatal(err)
    }
    resp.Body.Close()

    log.Println(resp.Status, resp.Body)
}