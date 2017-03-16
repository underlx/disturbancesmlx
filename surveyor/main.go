package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const statusURL = "http://app.metrolisboa.pt/status/estado_Linhas.php"

var previousResponse []byte

func main() {
	for {
		response, err := http.Get(statusURL)
		if err != nil {
			log.Println(err)
			continue
		} else {
			func() {
				defer response.Body.Close()
				// making sure they don't troll us
				if response.ContentLength < 1024*1024 {
					content, err := ioutil.ReadAll(response.Body)
					if err != nil {
						log.Println(err)
						return
					}
					if !bytes.Equal(content, previousResponse) {
						log.Printf("New status with length %d\n", len(content))
						err := ioutil.WriteFile("mlstatus-"+time.Now().UTC().Format(time.RFC3339), content, 0644)
						if err != nil {
							log.Println(err)
							return
						}
						previousResponse = content
					}
				}
			}()
		}
		time.Sleep(60 * time.Second)
	}
}
