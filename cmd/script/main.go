package main

import (
	"bytes"
	"cf/internal/core"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

func main() {
	argIps := flag.String("ips", "", "Comma-separated list of IP ranges (CIDR or hyphen format)")
	argHost := flag.String("host", "", "Host header to use in requests to IPs")
	argUrl := flag.String("url", "", "Reference URL to compare against")
	argLookForText := flag.String("look-for-text", "", "Text to look for")
	argTimeout := flag.Int("timeout", 10, "Request timeout in seconds")
	argWorkers := flag.Int("workers", 50, "Number of concurrent workers")
	flag.Parse()

	core.MustNotBeEmpty("'host' must not be empty", *argHost)
	core.MustNotBeEmpty("'ips' must not be empty", *argIps)

	argUrlParsed, err := url.Parse(*argUrl)
	if err != nil {
		_ = core.Fail(fmt.Errorf("malformed url: %v", err))
	}

	ipsToCheck, err := core.IPParseRanges(*argIps)
	if err != nil {
		_ = core.Fail(fmt.Errorf("error parsing ips: %v", err))
		return
	}
	if len(ipsToCheck) == 0 {
		_ = core.Fail(fmt.Errorf("no IPs to check"))
		return
	}
	fmt.Printf("%d IPs to be checked\n", len(ipsToCheck))

	ctx := context.Background()
	filePrefix := "cf_" + time.Now().Format("20060102_150405")

	http.DefaultClient.Transport = &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives: true,
	}

	var desiredResponse = []byte(*argLookForText)
	if *argLookForText == "" {
		desiredResponse, err = makeHttpRequest(time.Duration(*argTimeout)*time.Second, "", *argUrlParsed)
		if err != nil {
			_ = core.Fail(fmt.Errorf("error grabbing desired response: %v", err))
			return
		}
	}

	fileErrors := core.MustCreateFile(fmt.Sprintf("%s_errors.csv", filePrefix))
	defer fileErrors.Close()
	fileFoundIps := core.MustCreateFile(fmt.Sprintf("%s_found_ips.csv", filePrefix))
	defer fileFoundIps.Close()
	fileCheckedIps := core.MustCreateFile(fmt.Sprintf("%s_checked_ips.csv", filePrefix))
	defer fileCheckedIps.Close()
	fileProgress := core.MustCreateFile(fmt.Sprintf("%s_progress.json", filePrefix))
	defer fileProgress.Close()
	fileLookingFoResponse := core.MustCreateFile(fmt.Sprintf("%s_looking_for_response", filePrefix))
	_, _ = fileLookingFoResponse.Write(desiredResponse)
	_ = fileLookingFoResponse.Close()

	numIpsChecked := atomic.Int32{}
	numIpsErrored := atomic.Int32{}
	numIpsFound := atomic.Int32{}
	chanErrors := make(chan [2]string, 50000)

	parallel := core.NewParallel(ctx, *argWorkers)
	parallel.Schedule(func() error {
		for i := range ipsToCheck {
			errRun := func() error {
				return parallel.Run(func() error {
					defer numIpsChecked.Add(1)

					_, _ = fileCheckedIps.WriteString(ipsToCheck[i] + "\n")

					urlForRequest := *argUrlParsed
					urlForRequest.Scheme = "http"
					urlForRequest.Host = ipsToCheck[i] + ":80"

					responseBody, errMakeHttpRequest := makeHttpRequest(time.Duration(*argTimeout)*time.Second, *argHost, urlForRequest)

					if errMakeHttpRequest != nil {
						chanErrors <- [2]string{ipsToCheck[i], errMakeHttpRequest.Error()}
						numIpsErrored.Add(1)
						return nil
					}

					if bytes.Contains(responseBody, desiredResponse) {
						numIpsFound.Add(1)
						_, _ = fileFoundIps.WriteString(ipsToCheck[i] + "\n")
					}

					return nil
				})
			}()
			if errRun != nil {
				return errRun
			}
		}

		return nil
	})

	go core.Periodic(ctx, time.Second*10, func() error {
		ctxCollect, ctxCollectCancel := context.WithTimeout(ctx, time.Second*5)
		defer ctxCollectCancel()

	loop:
		for {
			select {
			case <-ctxCollect.Done():
				break loop
			case rec := <-chanErrors:
				_, _ = fileErrors.WriteString(fmt.Sprintf(`%s, "%s"`, rec[0], strings.ReplaceAll(rec[1], `"`, `\"`)))
				_, _ = fileErrors.WriteString("\n")
			}
		}

		_ = fileErrors.Sync()
		return nil
	})

	go core.Periodic(ctx, time.Second*10, func() error {
		_, _ = fileProgress.Seek(0, 0)
		_, _ = fileProgress.WriteString(fmt.Sprintf(
			`{"checked": %d, "errored": %d, "found": %d, "left": %d}`,
			numIpsChecked.Load(), numIpsErrored.Load(), numIpsFound.Load(), len(ipsToCheck)-int(numIpsChecked.Load()),
		))
		_ = fileProgress.Sync()
		return nil
	})

	_ = core.Fail(parallel.Wait())
	time.Sleep(time.Second * 11)
}

func makeHttpRequest(timeout time.Duration, host string, url url.URL) ([]byte, error) {
	ctx, ctxCancel := context.WithTimeout(context.Background(), timeout)
	defer ctxCancel()

	request, _ := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	request.Header.Set("Host", host)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return responseBody, nil
}
