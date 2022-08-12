package main

import (
	"fmt"
	"github.com/cavaliergopher/grab/v3"
	"sync"
)

type DownloadContext struct {
	username string
}

type DownloadRequest struct {
	Url     string           `json:"url"`
	Path    string           `json:"path"`
	Context *DownloadContext `json:"context"`
}

type DownloadResult struct {
	response *grab.Response
	path     string
	context  *DownloadContext
}

func (d DownloadResult) String() string {
	return fmt.Sprint(d.response.HTTPResponse.Request.URL, d.response.Filename, d.response.HTTPResponse.StatusCode)
}

type Downloader struct {
	routinesCount int
	client        *grab.Client
	requestChan   <-chan *DownloadRequest
	responseChan  chan<- *DownloadResult
}

func initDownloader(
	routinesCount int, requestChan <-chan *DownloadRequest, responseChan chan<- *DownloadResult) *Downloader {
	downloader := Downloader{
		routinesCount: routinesCount,
		client:        grab.NewClient(),
		requestChan:   requestChan,
		responseChan:  responseChan,
	}

	var wg sync.WaitGroup
	for i := 0; i < routinesCount; i++ {
		wg.Add(1)
		go func() {
			for req := range downloader.requestChan {
				Download(downloader.client, req, func(resp *grab.Response, context *DownloadContext) {
					downloader.responseChan <- &DownloadResult{
						response: resp,
						path:     req.Path,
						context:  context,
					}
				})
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(responseChan)
	}()
	return &downloader
}

func Download(client *grab.Client, request *DownloadRequest, callback func(resp *grab.Response, context *DownloadContext)) {
	req, err := grab.NewRequest(request.Path, request.Url)
	if err != nil {
		fmt.Println(err)
	}
	// start Download
	resp := client.Do(req)
	<-resp.Done
	callback(resp, request.Context)
}
