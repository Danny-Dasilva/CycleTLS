const initCycleTLS = require("../dist/index.js");
// import initCycleTLS from '../dist/index.js'
// Typescript: import initCycleTLS from 'cycletls';

(async () => {
  // Initiate CycleTLS
  const cycleTLS = await initCycleTLS();

  let data = JSON.stringify({
    destination: "test@gmail.com",
    country: "IT",
    type: "PASSWORD_RESET",
    client_id: "4fd2d5e7db76e0f85a6bb56721bd51df",
    language: "de",
  });
  console.log(data)
  // Send request
  const response = await cycleTLS(
    "https://accounts.nike.com/verification_code/send/v1",
    {
      body: data,
      ja3: "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
      userAgent:
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.0.0 Safari/537.36",
      headers: {
        "authority": "accounts.nike.com",
        "accept": "*/*",
        "accept-language": "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7",
        "cache-control": "no-cache",
        "content-type": "application/json",
        "cookie":
          `did=1fef32f0-514e-444d-89e2-ac1f1e3e90b1; NIKE_COMMERCE_COUNTRY=DE; s_ecid=MCMID%7C24477856746318886877663957399105420127; AMCV_F0935E09512D2C270A490D4D%40AdobeOrg=1994364360%7CMCMID%7C24477856746318886877663957399105420127%7CMCAID%7CNONE%7CMCOPTOUT-1659721166s%7CNONE%7CvVersion%7C3.4.0; sq=3; anonymousId=7C2F9A0B271C3EE439D3C6B352C6BB1C; AnalysisUserId=88.221.196.245.34331659713979990; _gcl_au=1.1.1417063997.1659713989; _scid=59588c96-2f8d-43c5-b212-6d8a2c1311d8; _fbp=fb.1.1659713991094.693136496; feature_enabled__as_accounts_cutover=true; NIKE_COMMERCE_LANG_LOCALE=de_DE; _pin_unauth=dWlkPU16bG1OMlprTldNdE5qTTVZUzAwWWpKaUxXRmtNVFF0WkdNeVpERm1Zalk0TmpGaw; bc_nike_triggermail=%7B%22distinct_id%22%3A%20%221826ea9b260b95-0b13c2822ba886-1c525635-13c680-1826ea9b261d47%22%2C%22bc_persist_updated%22%3A%201659722136632%2C%22cart_total%22%3A%200%2C%22bc_id%22%3A%20-172019985%2C%22bluecoreSiteIsMember%22%3A%20true%7D; cto_bundle=bx6_8V9ZeTN2Szdwa2xRT2IwWFR2WkVCUFZOY2VlJTJGUXZvZDhjJTJCTURINkJtQktIZzNoWHprYWNHamFtN0NFaFdNbjRRSUpWJTJCVkJtMUtOWlA4YjdTeExvMVNJS2Z5b0R2TnVLc3Z3TDZLYyUyRk1UVmFoWWdlNnclMkZqbEwxdVU4d1NTRVJCJTJCWkhtUnZRWlE5MnNhVHNPSGJleVhRZzh5eHRubU1QJTJGYjZQWEhscThsWnNFRlJ5VXJZUTZXMXF3akd5bXlrQTY1Rw; _ga_QTVTHYLBQS=GS1.1.1659722135.1.0.1659722137.58; _gid=GA1.2.454031510.1660055034; AKA_A2=A; geoloc=cc=IT,rc=,tp=vhigh,tz=GMT+1,la=45.47,lo=9.20; audience_segmentation_performed=true; feature_enabled__as_nav_rollout=true; bm_mi=F9A7F529B7F4A84A4A9AF0244565BF59~YAAQzRXfrYze3GaCAQAAiAeygxC1EOzK9LQ6YdRtiA28dMPEVmdF6Ilk0DsGHKWMRcs9jwaqwCnTuwvaSa44PhnFzREOSTBDBlFMwjyWjBkM+A9QmdxZFegKupQ3RCtYwEIaAtdUzDpDdNFdZh0NIhWsEX7Ti4Si/oFeblvKgwnmr9CQHY0DXFxm6gVq75crP7SeSJmZDCHnSvWUU3uUkF8M/322RRaFXCpW6E3a6nXoJKFMqpo+QQqHaphEez9NKtWcEoEUrkuf15VaGDLpJx1M7PJV7CO9+oRKBxvaqS9dTBE0b4mdZkT6xpcf9PSibk0sTE8=~1; ak_bmsc=2B9A6A5408BD7008DB7C3AA522C2ADA3~000000000000000000000000000000~YAAQzxXfreCZ2UyCAQAAWgqygxAMvyCKmNtO0nYH4k7cgy0VM1/ge03mU9F9gRdl9h/r+Mtua/Gr4QagEZR1Hla2NDObCvNcIduGe6eiSywpTTjNHUGw9e4HTg9+Irmd0/71qwd5m0EBwwAsGEDkE0vSAuiUfw3sIIZmTdzJcECUyrGFXFcJZe694z0QGlYRX0uROUIgm8NXh/BaLsg0mC4FBsGshznbPuFuEepDwb67h7ylmIrMl7iFnJVM6agfamhfTRQWlQDhBF5UpOQ/igiTSnz+zFH5JR6MCg+roIICnfSkg4gnqWXHi01y5TTwOutiWYlPCHu9lwhBTSKGoZyIHiwQqFL5u2f4PTgumdvNWPr+THIcW+8u64EzMcbv8jXwCx5jcve8PRTusmQnWGB92pEPzYVSiIsP7nn0jzQpL5iYMTcMs2d/ZwOKEKIw4HyoRNNPRe8TYtVj8kRUlCYZRqy3TC7urFkku5QUqVy122s+4w9I39fxncA=; ppd=homepage|nikecom>homepage; emperor_id=c33fc5da-5086-4166-a4ad-d586f53c763d; _gat_UA-167630499-3=1; _ga=GA1.1.310247532.1659713990; bc_nike_germany_triggermail=%7B%22distinct_id%22%3A%20%221826ea9b260b95-0b13c2822ba886-1c525635-13c680-1826ea9b261d47%22%2C%22bc_persist_updated%22%3A%201659907155616%2C%22cart_total%22%3A%200%2C%22bc_id%22%3A%20-172019985%2C%22bluecoreSiteIsMember%22%3A%20true%7D; _uetsid=e6aa8a0017ee11ed98678b0438bb41bb; _uetvid=d8235f5014d411edb67f77dfaa8efe17; ak_wfSession=1660068858~id=kKYgbOWzRkrjbKRNH7c2msMejmWRV5NQf++lER1S1Z0=; RT="z=1&dm=nike.com&si=e7d6d81a-8e4d-459f-ace1-61ea842ffe27&ss=l6mi1lar&sl=1&tt=45f&bcn=%2F%2F684dd325.akstat.io%2F&ld=45o&ul=qy6&hd=ug0"; _ga_EWJ8FE8V0B=GS1.1.1660068711.8.0.1660068742.29; ak_bmsc_nke-2.2-ssn=02im3X6yzX1juzf7ayv4cblL34TLoLPbeo52byISMd2XF7la24gxOBfuPHrSMUcs1rI3dNLhRlGFW2KmWetrWc7Ra6h5eQbd8HxI8FKDdMV12UuvuBxr8Er8mqoNSLzVroqagsPTkzULqozYNqyW9zXu17M; ak_bmsc_nke-2.2=02im3X6yzX1juzf7ayv4cblL34TLoLPbeo52byISMd2XF7la24gxOBfuPHrSMUcs1rI3dNLhRlGFW2KmWetrWc7Ra6h5eQbd8HxI8FKDdMV12UuvuBxr8Er8mqoNSLzVroqagsPTkzULqozYNqyW9zXu17M; _abck=D568D22762A8285A72364D492BBF57EC~-1~YAAQLWjerb6yImSCAQAA+xLPgwivCuB3pkAvUndSIf5wGISuKXScw0XZ2OzAZQAu7UL7N0vbcweO4pWt1tXcURi1UX1OGKrACPweVl/+BIebskTKsIndYQGyfrC2YKorf2mW6IgOn3XemnVWa8Mz131ri/RDE2Agi/mRSxz+sXokVPUF9cUozOckSLXgD9/KZRr1DCubVGvYoE8sh8PLD9T8+oelXhjTzpe0U+/3Eid/PsoqM7mQf8KDCyNPl46mxGnhSv8twKNJ60nV/JkvPjyKqUukBN5E8nAghFIV343fA2+MzCteDgFVd1MmuLYXkX1EJqR8GUkFoC/YArre1SRxwZfIrcSV8X95xoEsESS/U822Mpft2MyLjvTQB8lFXWDRpdey8kczVRXdT+8q/qBxJL/WNqO9e8rn2FYCnOwcdf9T9Qin++dmJ1Vjzj25yGCXdHEUrcVXwC1lEIVjrwyNjql7jhr9+rJx50ZXUhQ0sSeoh+cN1rSNy9Xva/QjmcFjRpj0fA==~0~-1~-1; bm_sv=FBE213F3C940D148B16C3DD4321AB85E~YAAQLWjerb+yImSCAQAA+xLPgxComm5+aFN41JILdrScaMrD2pfyQUwHWqvBfNHwmeBDjwDciPYt6JKVCqW0U2CkY4cm6eyx+3rIWRZ+EEwqvQQFdl74KJfyvl6PeU0leaQR4ravr69nJrEDxxwO8TSNNDrH3tWpj8b9CzWhwhXbiQdung/JTpCFn7hoSfWfHyLModiQAZlmKuoLRUo2Kv7boMm19qTSzQDKV34Wj3mEbl3JPU1bbXP1PNSx5eg=~1; bm_sz=38D46FB4B69DFC7D0278FCB9FC0D039A~YAAQLWjercCyImSCAQAA+xLPgxD2CImB/+lGo9N3Hy+NvYtuT7l/JPEbiGbWlnCbAZPv+2tjWUtYKZ0++GqrwzQIogiUFD4eCSj2/NSMm0CuixKDd5zH4/oNXSTMHAzzVKKhk0jovMFjGIASvEmUFZzOnpOJUvjwg+hOHoSH8g2347KAyKbWAQI+mWPjQz+sPWQLFrEGpRD7EI4VBUVF77zwOIbaSy8qKFG5sjXyDBq4xIcIKRIdH95h6g/jaToyriyJ8SoyIv+FuRH7NB2z7aMQPpvTGR6YPOa7a1mWCIxoEHK1hBrn0c8ot76hOaonuHw8K3AhjPR6AMLazarUCBil1PfbdgpYD2054omUixlH1KsKtIC2ztrMkdKxnKCX4MLPqyyTYpCzBWnOI23BGgrUUfx+I6LN+DDa1pB9F2paQdYeaxdhRHGSdA==~4469304~4272432; _abck=D568D22762A8285A72364D492BBF57EC~-1~YAAQOGjeraeai1yCAQAAjo/SgwgpMxISieF1IB9hrsqygDTWl1LXbdQ7HAdJyjD16oqqNQxFbxdhj6LhuAWBPtIVvz5OsQIu1+g4uAXTocbcviyObQR7qHZPUY4HdcR91JXe1c2PpeQrD+XBu30jjVMDrJOKOpkmHOjFoO2+OFykYd1ObLTsbSWWV4ZE6zzmqjCIMi+ubokmOF1u/5Wa9W/w8IU4Ti15tBJxKRae+EP0MvFIgqzlHz2MVQmvJ60OZby0EO1jClFv3S0rsdANwiaIRKXnylMh1VyGw3m0fNmU3ChSenGv2fGLlC2chQ695/Le/YwaX4RnRBPijFbeoyfh8p67Qbwpjz7x2wM4vzrNIK3FD1KyBRTQ0Z2GNm1zzv06I8B5fBfDTx3qmP/T6A2Qgn+W5FWpi/FI+YH/9ea0qgawJWBNsqDsPxO+W8RS638Mc+p+nzeIWOYQ8aOKd9q6w2h7MYeP2zGJ+yhYec8RLDgBmpU/FbZxWDFXrMwmIT5S4E9xWdpNRLaDBB0GsOqUUdvKCajn3w==~-1~-1~-1; ak_bmsc=4600DD5B59C71814E61507340FFFFEFF~000000000000000000000000000000~YAAQRv8SAnGie3SCAQAAj7WwgxBWMkoxAOBJq7GaofIveYClEt1Avg3UG6Ywk733X3zyvfy1pRHmWwAdTXEx49gm/GXVrhFkNBLgEXOhAd4g8D1A9OJUhiU3TCQUNStQ7N2hV5TGTrObAasJuUuL/5Mjl9IjD89SCtNrCD53GCP9LX2SfPpL/IUxzMht9htwi8OXikGImMbTYxzfOrn6KrFibhSkEOdWjdIVFp4iN11a7QJVBI1yHRl9HmxV9XcC22feLz3upFK9FtSl/BKW0JkapM+qYLDShHU2hQlyJzGklKH9A69DZwD/+shi3XQSXvDBdRyaqEzpJ9kkkASNPVjl/WOuAr5pCdwsxLK2oHgEnC59mEc5/bsrYg==; bm_sv=C365FBA0F6ECD46CDD74D27163056A94~YAAQOGjerX2Xi1yCAQAAHMnRgxCMkNAMfssb73j6SSPsFK2Ed7OZ1Bb7kDY65dAasc19tnOeKpVzXFNsTwKaet3/PVm46RCyuSbW5ckQw/GCimRMbCa6zh3CeOMGEWanpqlfQ92wrtQSNTH4Bq5vNejZ5SJYvVCwM/VF2W8l0d5RmefnLQ4dEoV4LdsTJBJASgWCGNKhivFmkoEi1BQWdco2+TH7IOiImz5Hi+ponwrzbMLXG7p73lXBg4CF2g==~1; bm_sz=1C27C7533A758D36E9DCF98BF03C6112~YAAQRv8SAnKie3SCAQAAj7WwgxCC9Rp2j3NQInU0+Y1m11WzSX/iKnQ+GLpP76NaW3fdmSgWUBvULGEbixtyXnLDNPR3UQVXg40nvlzkhgdRYAmp+q+tR7rp0qqiOBamSarHfPO7cq1I5JkTqQNw2pePCqRgRA3K10/EZ8MrBp61xlPTQc0ffiBYgSrYejUOYAx7h+q+GkFBPsAilvyezmZirccdrO5hUPkc7JKD4mUJLVwVkX0DfGQ4OqgeAX7+oNGZF5xa258/wL1ksyaHTnhz1VQYQ8/x58gfhdLyjuKUf36Q6qStyjTcPAkbijpfm9qVYCa1zjtgAv8To/P2JsAvYMvWN9+DWPQkuKjA0cU5h3lUwcimu+OQsOGO/C3EnXwpu+CfHVDKKRG8jLvEJrVptA==~4536643~3617089; ak_bmsc_nke-2.2=02oGaq2cuSrr6OwwXID1XFOpnpOu1PxUwp5zCnAmRD1bSvO0KZKTkH5fdLZbMdUp2j8O6N7AJsr3YDsOvU5JGDm5cxAfqxOTdrOCXPSuLNUaCe0ZRTNa92uUNd3oGQE2Q7wxYBIDC97CUAalwcMYoy1Pooz; ak_bmsc_nke-2.2-ssn=02oGaq2cuSrr6OwwXID1XFOpnpOu1PxUwp5zCnAmRD1bSvO0KZKTkH5fdLZbMdUp2j8O6N7AJsr3YDsOvU5JGDm5cxAfqxOTdrOCXPSuLNUaCe0ZRTNa92uUNd3oGQE2Q7wxYBIDC97CUAalwcMYoy1Pooz`,
        "newrelic":
          "eyJ2IjpbMCwxXSwiZCI6eyJ0eSI6IkJyb3dzZXIiLCJhYyI6IjEwMTU4MTAiLCJhcCI6IjQ2NDA0NjI1NCIsImlkIjoiZDBmODdmOGUxNTU2NTU0ZSIsInRyIjoiNDM4Y2M4NDM0NzU4MjZlOWQ3ZDRlNGU2YmZlODAxYTUiLCJ0aSI6MTY2MDA2ODc2OTE3NiwidGsiOiIxNjMxNTE4In19",
        "origin": "https://accounts.nike.com",
        "pragma": "no-cache",
        "referer":
          "https://accounts.nike.com/set-password?client_id=4fd2d5e7db76e0f85a6bb56721bd51df&redirect_uri=https://www.nike.com/auth/login&response_type=code&scope=openid%20nike.digital%20profile%20email%20phone%20flow%20country&state=e979037416c44f3cb6e13db2c0e5cb9d&code_challenge=32_kVdvlcelOuMlutx4gBYj2qkpvGH6qaGR-JAAQ4ms&code_challenge_method=S256",
        "sec-ch-ua":
          '".Not/A)Brand";v="99", "Google Chrome";v="103", "Chromium";v="103"',
        "sec-ch-ua-mobile": "?0",
        "sec-ch-ua-platform": '"macOS"',
        "sec-fetch-dest": "empty",
        "sec-fetch-mode": "same-origin",
        "sec-fetch-site": "same-origin",
        "traceparent": "00-438cc843475826e9d7d4e4e6bfe801a5-d0f87f8e1556554e-01",
        "tracestate":
          "1631518@nr=0-1-1015810-464046254-d0f87f8e1556554e----1660068769176",
        "user-agent":
          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.0.0 Safari/537.36",
        "x-kpsdk-cd":
          '{"workTime":1660068769096,"id":"b2df473a5d6a2079bc2b791ffdb32afa","answers":[9,8],"duration":5.4,"d":74,"st":1660068748127,"rst":1660068748201}',
        "x-kpsdk-ct":
          "02im3X6yzX1juzf7ayv4cblL34TLoLPbeo52byISMd2XF7la24gxOBfuPHrSMUcs1rI3dNLhRlGFW2KmWetrWc7Ra6h5eQbd8HxI8FKDdMV12UuvuBxr8Er8mqoNSLzVroqagsPTkzULqozYNqyW9zXu17M",
      },
      timeout: 30,
    },
    "POST"
  );

  console.log(response);
})();
