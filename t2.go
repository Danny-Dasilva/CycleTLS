package main

import (
	"./cycletls"
	"log"
	"runtime"
	"time"
	// "net/http"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	start := time.Now()
	defer func() {
		log.Println("Execution Time: ", time.Since(start))
	}()
	client := cycletls.Init()
	response, err := client.Do("https://api.coinbase.com/v2/mobile/users/", cycletls.Options{
		Body:      `{\"locale\":\"en\",\"password\":\"minayaggag5€\",\"promo_code\":\"\",\"accept_user_agreement\":true,\"response_type\":\"code\",\"last_name\":\"Ababab\",\"application_client_id\":\"6011662b0badfa97f9fed5a246526277ff2116affa98cfaacacd012a191ba38d\",\"email\":\"test@gmail.com\",\"threatmetrix_session_id\":\"93617af0542a4192bb3016dd085be46c\",\"first_name\":\"Ajanna\"}`,
		Ja3:       "771,4865-4866-4867-49196-49195-52393-49200-49199-52392-49162-49161-49172-49171-157-156-53-47-49160-49170-10,0-23-65281-10-11-16-5-13-18-51-45-43-27,29-23-24-25,0",
		UserAgent: "Coinbase/9.55.3 (com.vilcsak.bitcoin2; build:95503; iOS 15.5)",
		Headers:   map[string]string{
			"x-cb-version-name": "10.19.2",
			"x-cb-platform": "ios",
			"User-Agent": "Coinbase/10.19.2 (com.vilcsak.bitcoin2; build:10190002; iOS 15.5.1)",
			"x-firebase-app-instance-id": "FA4A00B9B8C44C99B54910E244D75714",
			"cb-client": "com.vilcsak.bitcoin2/10.19.2/10190002",
			"recaptcha-token": "03AGdBq27YMTt8RNL8iLEpgSR7w2wSFm1zcNF1U0pR9dvY9pN1T42JveyUg3kudDXcGxeodg1NjzaO8mYUHGsk32U4d6CzG5l7hrRoMxgEYjw-xl9Ektfe4pnqeBoCeqkKzvR1RqaGa4epeXwlHCXO0aqenPvufOeNu0wazeBbbMbOxxJXn6PvkbCxklcCN4nTre5DUXN-GY3SHj3iwTcrtyCgU62QWkwQGverjc8Qgwe8Ltirm-_v_hgHfhz3Hh_YeLksK8HImBVkLb_zJZaqB26FsFOpxG0ADJPjCCLSj_Hm92aAfIOvKf5gGZYiuJ7a80cZfHy1YruFok4iC9HH9AQQpnk5doiHxp_pdBMxP1PAFaux8hjzU-cJ9mbjfBiaO7udz3TJBBlTpliPTNrObcUubCKvHJwWfMXA-WJg18yPpfmCe2VQp6Id6Ira2Icmzj7UgNZfmciHP8IVjCN8Iq1AVsZA45__2_fysV1hUj5YY-zT44CCbhCXF0csLQXV-sLGgfbBwGHsWuwGqv7aKLen9vxtk8pVrnqq_uIUc51iX2wolnaz4qaAk1ENT1cIuhrbsnhBnhGUflx_i1AKBLqG_sjefPs_tlfj0vybYTOcXoqlh9SGt4Ix1jbe-qBrE4vO3U8g-P837lSH7OLlmCJHRwqR5xAA9xozJCe4liKaLr5UuQbPmyZTiCrqsmYqRQvqBCOY7DDhSOJnLbjaXDOxkEH4Hh58YOJVeMJXWpzpu1AH5IqUCVRS5LNDi7y0VNOmbKQmZ8nmjvrIa90Ol9EXhJoMhcBolotfgjyTgDet5SXpZ5C6tv7wNf7j30y4tnnnDHt_OKAG91K531jqZ8nQceixpqIA6L5RODF2-wCIsJYVVtn-lk8fYSVTVn8BYi5zSTeae7FKAywjyd3Dh-gwdI-loTa4sWpYCPpN2Xytn5Ei8NWEuy72P41QB4beMkGACMXtuyTLVVR6ADxQS5PErMEjFNVwwCaJ-0wJq-we3nq-OzqDW6YL1agiXmHKi2rnAVwjXyIElnvBa5NX_tnF0Ac-WIofB_O-AaP42L-v44zz9Mgx_Yw2tJic42deam9p4EsODKJqdiEScy6xNZZPv6sppH7OsKzDVfIQSUK-b2xkmo-iu_Jrcu-rx6X84I8svs7jWRtjJr5BwysKzpoW3POn-LcLR_AUOYPLegKujcKMx71jfkTaRNpnI6mLLjchfF7VsU5Alc6x1lo23TqcfUp06R1ylfxkJ8h-au9lPvBZW7_W8DwT8s4oHSvVnGNth4HS0SY88UboviW5XNvTPXqgt1zjBTDQuMf3IP3eCbCDztQ77qWkGOO7N4bOUo0NjzqWWktAN2CgMu1xiftFKa_RNWmY_JV3iiwD5G3RYlcnx_C-AqZeDStSSs4jXQ6mYEK7llRwo-WoKfr8I6W0mQN8ExwccdYUrvnKDHdfy4k_kjc42VrJ1S5J21CORD1FDT0VLCtezYbQgk5rqaKTI-ZXXIKCDJSIAlbHLOtZPgFx10S6tk8mOkrWZoD50ZaUpQFyAanLhUjT3AcxqBOJzKgBAr-SLWASLWnn1OJVv69KeYFA5zc7LVMfK5uD9SNLcfbHqfKMdHi5UDkpHhiCyr5kA5Q3chEzvE56XoE9b74RFFwt1m5YLl-dfJ2SSjk44KmHmwsDv5M7m4HplP5JMWyokfXT49JVg00Hj8a8X4S0BKOmJkmUnOblz5fNOzDPdNJWXVKh0TflXjbZsdMGQG6Hs_hJXQzqxbZ9l-UnqHI6z1CJBtvKExIQipEVb6q8vXGCP7kCkK3GcEz6o0zPjjXM49GDZA",
			"x-cb-session-uuid": "86266f82-d675-4047-aa5e-4afc2c4a2bcc",
			"cb-version":"2021-01-11",
			"x-cb-project-name": "consumer",
			"Content-Length": "331",
			"x-appsflyer-id": "1624858310904-8516891",
			"x-cb-is-logged-in": "true",
			"Connection": "keep-alive",
			"cb-fp2": "691A0B45-2FF0-463C-8702-68BEC9AA3D03",
			"Accept-Language": "en",
			"Accept": "application/json",
			"Content-Type": "application/json",
			"Accept-Encoding": "gzip, deflate, br",
			"x-cb-device-id": "691A0B45-2FF0-463C-8702-68BEC9AA3D03",
			"x-cb-pagekey": "onboarding",

	},

	}, "POST")
	if err != nil {
		log.Print("Request Failed: " + err.Error())
	}
	log.Println(response.Body)

}
