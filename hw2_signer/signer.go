package main

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"sync"
)

func SingleHash(in, out chan interface{}) {
	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}
	for dataRaw := range in {
		dataInt, ok := dataRaw.(int)
		if !ok {
			fmt.Println("SH: cant convert result data to int")
		} else {
			dataStr := strconv.Itoa(dataInt)
			wg.Add(1)

			go func(inStr string, mU *sync.Mutex, wG *sync.WaitGroup, out chan<- interface{}) {
				defer wG.Done()

				resultBare := make(chan string, 1)
				go func(inStr string, out chan<- string) {
					out <- DataSignerCrc32(inStr)
				}(dataStr, resultBare)

				mu.Lock()
				dataStrMd5 := DataSignerMd5(dataStr)
				mu.Unlock()

				resultMd5 := make(chan string, 1)
				go func(inStr string, out chan<- string) {
					out <- DataSignerCrc32(inStr)
				}(dataStrMd5, resultMd5)

				bareStr := <-resultBare
				md5Str := <-resultMd5
				out <- bareStr + "~" + md5Str
			}(dataStr, mu, wg, out)
		}
	}
	wg.Wait()
}

func MultiHash(in, out chan interface{}) {
	type retPair struct {
		idx int
		res string
	}

	wg := &sync.WaitGroup{}
	for dataRaw := range in {
		dataStr, ok := dataRaw.(string)
		if !ok {
			fmt.Println("MH: cant convert result data to string")
		} else {
			wg.Add(1)

			go func(inStr string, wG *sync.WaitGroup, out chan<- interface{}) {
				defer wG.Done()
				result := make(chan retPair, 6)
				for i := 0; i < 6; i++ {
					go func(inStr string, i int, out chan<- retPair) {
						out <- retPair{idx: i, res: DataSignerCrc32(strconv.Itoa(i) + inStr)}
					}(dataStr, i, result)
				}

				var resultArr [6]string
				for i := 0; i < 6; i++ {
					rp := <-result
					resultArr[rp.idx] = rp.res
				}

				var buffer bytes.Buffer
				for i := 0; i < 6; i++ {
					buffer.WriteString(resultArr[i])
				}

				out <- buffer.String()
			}(dataStr, wg, out)
		}
	}
	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	finalRes := make([]string, 0, MaxInputDataLen)
	for dataRaw := range in {
		dataStr, ok := dataRaw.(string)
		if !ok {
			fmt.Println("CR: cant convert result data to string")
		}

		finalRes = append(finalRes, dataStr)
	}
	sort.Strings(finalRes)
	var buffer bytes.Buffer
	for i := 0; i < len(finalRes); i++ {
		if i != 0 {
			buffer.WriteString("_")
		}
		buffer.WriteString(finalRes[i])
	}

	out <- buffer.String()
}

func ExecutePipeline(jobs ...job) {
	wg := &sync.WaitGroup{}
	inCh := make(chan interface{}, MaxInputDataLen)
	for _, j := range jobs {
		outCh := make(chan interface{}, MaxInputDataLen)
		wg.Add(1)
		go func(wG *sync.WaitGroup, j job, in chan interface{}, out chan interface{}) {
			defer wG.Done()
			j(in, out)
			close(out)
		}(wg, j, inCh, outCh)
		inCh = outCh
	}
	wg.Wait()
}
