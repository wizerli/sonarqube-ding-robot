package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	neturl "net/url"
	"strings"
)

var addr = flag.String("addr", "0.0.0.0:9001", "enter listening address")
var token = flag.String("token", "", "enter sonarqube token")
var httpClient = &http.Client{}

// 新增serverity相关metrics
func getMeasures(sonarUrl, projectKey interface{}) (MeasuresData, error) {
	var measuresData MeasuresData
	url := fmt.Sprintf("%s/api/measures/search?projectKeys=%s&metricKeys=alert_status,bugs,reliability_rating,vulnerabilities,security_rating,code_smells,sqale_rating,duplicated_lines_density,coverage,ncloc,ncloc_language_distribution,critical_violations,blocker_violations,major_violations,minor_violations,info_violations", sonarUrl, projectKey)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return measuresData, err
	}
	req.SetBasicAuth(*token, "")
	resp, err := httpClient.Do(req)
	if err != nil {
		return measuresData, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&measuresData)
	return measuresData, err
}

// buildDingLink 构造钉钉点击链接
// @url 要打开的链接
// @pcSlide true：表示在PC客户端侧边栏打开 false：表示在浏览器打开
func buildDingLink(url string, pcSlide bool) string {
	params := neturl.Values{}
	params.Add("url", url)
	sPcSlide := "false"
	if pcSlide {
		sPcSlide = "true"
	}
	params.Add("pcSlide", sPcSlide)
	return fmt.Sprintf("dingtalk://dingtalkclient/page/link?%s", params.Encode())
}

// dingTalkHandler
func dingTalkHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	var sonarQubeCallBackData SonarQubeCallBackData
	accessToken := r.Form.Get("access_token")
	if err := json.NewDecoder(r.Body).Decode(&sonarQubeCallBackData); err != nil {
		r.Body.Close()
		log.Fatal(err)
	}
	// sonar地址
	sonarUrl := sonarQubeCallBackData.ServerURL
	// 项目名称
	projectName := sonarQubeCallBackData.Project.Name
	// 项目key
	projectKey := sonarQubeCallBackData.Project.Key
	// sonar prop
	var totalBugs, vulnerabilities, codeSmells, coverage, duplicatedLinesDensity, alertStatus, blocker_violations, critical_violations, major_violations, minor_violations, info_violations string
	// dingtalk prop
	var sendUrl, picUrl, messageUrl, pdfUrl string
	
	
	// get sonar info
	measuresData, err := getMeasures(sonarUrl, projectKey)
	if err != nil {
		fmt.Printf("request measures error: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "request measures error: %v", err)
		return
	}
	for _, measure := range measuresData.Measures {
		switch measure.Metric {
		case "bugs":
			totalBugs = measure.Value
		case "vulnerabilities":
			vulnerabilities = measure.Value
		case "code_smells":
			codeSmells = measure.Value
		case "coverage":
			coverage = measure.Value
		case "duplicated_lines_density":
			duplicatedLinesDensity = measure.Value
		case "critical_violations":
			critical_violations = measure.Value
		case "blocker_violations":
			blocker_violations = measure.Value
		case "major_violations":
			major_violations = measure.Value
		case "minor_violations":
			minor_violations = measure.Value
		case "info_violations":
			info_violations = measure.Value
		case "alert_status":
			alertStatus = measure.Value
			switch alertStatus {
			case "ERROR":
				picUrl = "http://s1.ax1x.com/2020/10/29/BGMZwD.png"
			case "OK":
				picUrl = "http://s1.ax1x.com/2020/10/29/BGMeTe.png"
			}
		}
	}
	sendUrl = fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s", accessToken)
	messageUrl = fmt.Sprintf("%s/dashboard?id=%s", sonarUrl, projectKey)
	pdfUrl = fmt.Sprintf("%s/api/pdfreport/get?componentKey=%s", sonarUrl, projectKey)

	textList := []string{
		fmt.Sprintf("![head](%s)", picUrl),
		fmt.Sprintf("本次扫描仓库: %s", projectName),
		fmt.Sprintf("## 代码总体扫描结果"),
		fmt.Sprintf("BUG数: %s 个", totalBugs),
		fmt.Sprintf("漏洞数: %s 个", vulnerabilities),
		fmt.Sprintf("异味数: %s 个", codeSmells),
		fmt.Sprintf("测试覆盖率: %s%%", coverage),
		fmt.Sprintf("代码重复率: %s%%", duplicatedLinesDensity),
		fmt.Sprintf("阻断blocker数: %s 个", blocker_violations),
		fmt.Sprintf("严重critical数: %s 个", critical_violations),
		fmt.Sprintf("主要major数: %s 个", major_violations),
		fmt.Sprintf("次要minor数: %s 个", minor_violations),
		fmt.Sprintf("提示info数: %s 个", info_violations),
	}

	dingMsg := DingMsg{
		MsgType: "actionCard",
		ActionCard: DingActionCard{
			Title:          fmt.Sprintf("仓库 %s 的代码静态扫描结果", projectName),
			Text:           strings.Join(textList, "\n\n"),
			BtnOrientation: "0",
			Btns: []DingActionCardBtn{
				{
					Title:     fmt.Sprintf("点击查看分析结果WEB(需要浏览器登陆过sonarqube)"),
					ActionURL: buildDingLink(messageUrl, false),
				},
				{
					Title:     fmt.Sprintf("点击下载分析报告PDF(需要浏览器登陆过sonarqube)"),
					ActionURL: buildDingLink(pdfUrl, false),
				},
			},
		},
	}

	// send message
	paramBytes, _ := json.Marshal(dingMsg)
	response, _ := http.Post(sendUrl, "application/json", bytes.NewReader(paramBytes))
	fmt.Fprint(w, response)
}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func main() {
	flag.Parse()
	if !isFlagPassed("token") {
		fmt.Println("token参数是必须的")
		flag.Usage()
		return
	}
	http.HandleFunc("/dingtalk", dingTalkHandler)
	log.Printf("Server started on port(s): %s (http)\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
