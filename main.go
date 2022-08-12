package main

import (
	"fmt"
	"github.com/anaskhan96/soup"
	"gocv.io/x/gocv"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

func main() {
	requests := make(chan *DownloadRequest, 100)
	results := make(chan *DownloadResult, 100)

	needle := gocv.IMRead("needle.png", gocv.IMReadGrayScale)

	_ = os.Mkdir("pics", 0777)
	initDownloader(40, requests, results)
	var wg sync.WaitGroup
	for startPage := 0; startPage < 700; startPage += 50 {
		wg.Add(1)
		startPage := startPage
		go func() {
			for i := startPage; i < startPage+50; i++ {
				processPage(i, requests)
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(requests)
	}()

	sift := gocv.NewSIFT()
	defer func(sift *gocv.SIFT) {
		_ = sift.Close()
	}(&sift)
	_, needleDescriptor := sift.DetectAndCompute(needle, gocv.NewMat())
	score, matches := findMatches(&needleDescriptor, results)

	fmt.Println("##############################")
	fmt.Println("Best match score is ", score)
	fmt.Println("##############################")
	fmt.Println("Matches:")
	for i, match := range matches {
		fmt.Println(i+1, "\t", match.path, "\t", match.response.Request.URL())
	}
	fmt.Println("##############################")
}

func findMatches(needleDescriptor *gocv.Mat, results chan *DownloadResult) (int, []*DownloadResult) {

	var bestMatchResults = make([]*DownloadResult, 0, 1)
	var bestMatch = 0

	var ops uint64
	lastPrinted := uint64(0)
	var wg sync.WaitGroup
	var lock sync.Mutex
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sift := gocv.NewSIFT()
			defer func(sift *gocv.SIFT) {
				_ = sift.Close()
			}(&sift)
			for res := range results {
				matches := search(res.path, &sift, needleDescriptor)
				lock.Lock()
				if matches > bestMatch {
					bestMatch = matches
					bestMatchResults = make([]*DownloadResult, 0, 1)
					bestMatchResults = append(bestMatchResults, res)
					fmt.Println("BestMatch=", matches)
					fmt.Println(res.response.Request.URL())
					fmt.Println(res.path)
				} else if matches == bestMatch {
					bestMatch = matches
					bestMatchResults = append(bestMatchResults, res)
					fmt.Println("BestMatch=", matches)
					for _, result := range bestMatchResults {
						fmt.Println(result.path)
						fmt.Println(result.response.Request.URL())
					}
				}
				ops++
				if ops-lastPrinted > 500 {
					fmt.Println("Processed ", ops, " pics")
					fmt.Println("Last processed is  ", res.path)
					lastPrinted = ops
				}
				lock.Unlock()
			}
		}()
	}
	wg.Wait()
	return bestMatch, bestMatchResults
}

func search(path string, sift *gocv.SIFT, descriptor *gocv.Mat) int {
	hay := gocv.IMRead(path, gocv.IMReadGrayScale)

	_, des1 := sift.DetectAndCompute(hay, gocv.NewMat())
	matcher := gocv.NewBFMatcher()
	defer matcher.Close()
	matches := matcher.KnnMatch(*descriptor, des1, 2)
	res := 0
	for _, value := range matches {
		if len(value) != 2 {
			continue
		}
		if value[0].Distance < 0.7*value[1].Distance {
			res++
		}
	}
	return res

}

func processPage(pageNum int, requestChan chan *DownloadRequest) {
	url := "https://camfox.com/" + strconv.Itoa(pageNum)
	resp, err := soup.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	dirName := fmt.Sprint("pics/", pageNum)
	err = os.MkdirAll(dirName, 0777)
	if err != nil {
		panic(err)
	}
	doc := soup.HTMLParse(resp)
	mainContent := doc.FindStrict("div", "id", "list_videos_most_recent_videos_items")
	if mainContent.Error == nil {
		imgs := mainContent.FindAllStrict("img", "class", "thumb lazy-load")
		for _, img := range imgs {
			src := img.Attrs()["data-webp"]
			id := strings.Split(src, "/")[6]
			requestChan <- &DownloadRequest{
				Url:     src,
				Path:    fmt.Sprint("pics/", pageNum, "/", id, ".riff"),
				Context: nil,
			}
		}
	}
}
