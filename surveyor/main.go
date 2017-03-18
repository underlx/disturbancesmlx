package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
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
					var buf bytes.Buffer
					tee := io.TeeReader(response.Body, &buf)
					content, err := ioutil.ReadAll(tee)
					if err != nil {
						log.Println(err)
						return
					}
					if !bytes.Equal(content, previousResponse) {
						log.Printf("New status with length %d\n", len(content))
						err := ioutil.WriteFile("mlstatus-"+time.Now().UTC().Format("2006-01-02T15-04-05Z07-00"), content, 0644)
						if err != nil {
							log.Println(err)
							return
						}
						previousResponse = content

						doc, err := goquery.NewDocumentFromReader(&buf)
						if err != nil {
							log.Println(err)
							return
						}

						doc.Find("table").First().Find("tr").Each(func(i int, s *goquery.Selection) {
							line := s.Find("td").First()
							lineName := line.Find("b").Text()
							status := line.Next()
							if len(status.Find(".semperturbacao").Nodes) == 0 {
								// disturbance
								log.Printf("Disturbance on %s: %s\n", lineName, status.Find("li").Text())
							} else {
								log.Printf("%s: OK\n", lineName)
							}
						})
					}
				}
			}()
		}
		time.Sleep(60 * time.Second)
	}
}
