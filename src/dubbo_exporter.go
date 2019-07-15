package main

import (
    "fmt"
		"time"
		"net/url"
		"net/http"
		"strings"
		"log"
		"strconv"
		"flag"
		//"reflect"
		"github.com/Unknwon/goconfig"
		"github.com/samuel/go-zookeeper/zk"
		"github.com/prometheus/client_golang/prometheus"
		"github.com/prometheus/client_golang/prometheus/promhttp"
		//plog "github.com/prometheus/common/log"
		//"go.uber.org/zap"
)


//为程序配置confpath参数，获取用户指定的配置文件路径，默认为“config.ini”
var confpath string

func init() {
	flag.StringVar(&confpath,"confpath","config.ini","The config filepath")
}



//从zk读child信息，一次读一次关，用于读service列表
func readChildFromZK(zkhosts string,authstr string,readpath string) []string {
	var hosts = []string{zkhosts}
	conn, _, err := zk.Connect(hosts, time.Second*5)
	//用digest的方式来增加认证
	//若读取的字段不为空，则加认证
	if authstr != "" {
		authbyte := []byte(authstr)
		conn.AddAuth("digest",authbyte)
	}

	children, _,err := conn.Children(readpath)
	defer conn.Close()
	if err != nil {
		fmt.Println(err)
	}
	return children
}


//从配置文件中读取zk登陆配置信息
func readconf(filepath string) (string,string,string,string){
	cfg, err := goconfig.LoadConfigFile(filepath)
	if err != nil {
		fmt.Println(err)
	}
	host, _ := cfg.GetValue("zookeeper","host")
	auth, _ := cfg.GetValue("zookeeper","auth")
	rootpath, _ := cfg.GetValue("zookeeper","rootpath")
	prometheus_port, _ := cfg.GetValue("zookeeper","prometheus_port")
	return host,auth,rootpath,prometheus_port
}


//从zk读child信息，多次读一次关，用于读dubbo的列表
func readChildService(zkhosts string,authstr string,readpath []string) []string {
	var hosts = []string{zkhosts}
	conn, _, _ := zk.Connect(hosts, time.Second*5)
	
	//用digest的方式来增加认证
	//若读取的字段不为空，则加认证
	if authstr != "" {
		authbyte := []byte(authstr)
		conn.AddAuth("digest",authbyte)
	}

	var urlList []string
	for i:=1;i<len(readpath);i++{
		children, _, _ := conn.Children(readpath[i])
    for j:=1;j<len(children);j++{
			urlList = append(urlList,children[j])
		}
	}
	defer conn.Close()
	return urlList	
}


//将各个节点上被转义成url编码的节点转义回来
func ParseUrl(urlstr string) string{
	dubboprotocol, _ :=url.ParseQuery(urlstr)
	var dubbostr string
	for key, _ := range dubboprotocol {
		dubbostr =key
	}
	return dubbostr
}


//解析dubbo consumer或provider节点转成url后的细节信息，将地址，端口，接口，应用等解析出来
func ParseStr(urlstr string,roles string) []string{
	key1 := "application"
  key2 := "interface"
  key3 := "dubbo"
  key4 := "address"
  key5 := "port"
	key6 := "roles" //provider or consumer
	key7 := "status"
	vallist := strings.Split(urlstr,"&")
	//按顺序把值打入dubboList这个slice中，先生成map，按上述key的顺序，从map生成slice
	var dubboList []string
	dubbodic := make(map[string]string)
	//zk上检查到的都默认为在线状态，1，如果看到有降级标志，置为2
	dubbodic[key7] = "1"
	for i:=1;i<len(vallist);i++{
		switch {
			case strings.Contains(vallist[i], key1+"="):
				dubbodic[key1]=strings.Split(vallist[i],"=")[1]
			case strings.Contains(vallist[i], key2+"="):
				dubbodic[key2]=strings.Split(vallist[i],"=")[1]
			case strings.Contains(vallist[i], key3+"="):
				dubbodic[key3]=strings.Split(vallist[i],"=")[1]
			case strings.Contains(vallist[i]+"=", "mock"):
				dubbodic[key7] = "2"
			case strings.Contains(vallist[i]+"=", "override"):
				dubbodic[key7] = "2"
		}
	}
	dubbodic[key4]=strings.Split(strings.Split(vallist[0],"/")[2],":")[0]
	dubbodic[key5]=strings.Split(strings.Split(vallist[0],"/")[2],":")[1]
	dubbodic[key6]=roles
	//根据dubbodic按顺序打入slice中
	dubboList = append(dubboList,dubbodic[key1])
	dubboList = append(dubboList,dubbodic[key2])
	dubboList = append(dubboList,dubbodic[key3])
	dubboList = append(dubboList,dubbodic[key4])
	dubboList = append(dubboList,dubbodic[key5])
	dubboList = append(dubboList,dubbodic[key6])
	dubboList = append(dubboList,dubbodic[key7])
	
	return dubboList
}

//解析dubbo configurators节点转成url后的细节信息，将地址，端口，接口，应用等解析出来
func ParseConfigStr(urlstr string)[]string{
	tmpvallist1 := strings.Split(urlstr,"?")
	configvallist := strings.Split(tmpvallist1[0],"/")
	var addressstr,portstr,rolesstr,interfacestr string
	if strings.Contains(configvallist[2],":"){
		fmt.Println(configvallist[2])
		addressstr = strings.Split(configvallist[2],":")[0]
		portstr = strings.Split(configvallist[2],":")[1]
		rolesstr = "providers"
	} else {
		addressstr = configvallist[2]
		portstr = "null"
		rolesstr = "consumers"
	}
	interfacestr = configvallist[3]
	var tempList []string
	tempList = append(tempList,addressstr)
	tempList = append(tempList,portstr)
	tempList = append(tempList,interfacestr)
	tempList = append(tempList,rolesstr)
	return tempList
}


//将单条生产者or消费者URL信息解析
func ParseDubboUrl(urlList []string,roles string) [][]string{
	var dubboList [][]string
	for i :=0;i<len(urlList);i++{
		dubbostr := ParseUrl(urlList[i])
		tmpdubboList := ParseStr(dubbostr,roles)
		dubboList = append(dubboList,tmpdubboList)
	}
	return dubboList
}


//将单条configurator URL信息解析
func ParseConfigUrl(urlList []string) [][]string{
	var configList [][]string
	for i :=0;i<len(urlList);i++{
		configstr := ParseUrl(urlList[i])
		tempList := ParseConfigStr(configstr)
		configList = append(configList,tempList)
	}
	return configList
}

//收集dubbo的信息
func collecter(zkhosts string,authstr string,rootpath string) [][]string{
	serviceList := readChildFromZK(zkhosts,authstr,rootpath)
	var providerPath []string
	for i:=0;i<len(serviceList);i++{
		providerPath = append(providerPath,rootpath+"/"+serviceList[i]+"/providers")
	}
	dubboUrlList := readChildService(zkhosts,authstr,providerPath)

	var dubboServiceInfo [][]string
	dubboServiceInfo = ParseDubboUrl(dubboUrlList,"providers")

	//查一下有没有configurators里面有内容的，有的为降级
	var configuratorPath []string
	for i:=0;i<len(serviceList);i++{
		configuratorPath = append(configuratorPath,rootpath+"/"+serviceList[i]+"/configurators")
	}
	configUrlList := readChildService(zkhosts,authstr,configuratorPath)

	var dubboConfigInfo [][]string
	dubboConfigInfo = ParseConfigUrl(configUrlList)
	fmt.Println(dubboConfigInfo)
	
	return dubboServiceInfo
}

//定义prometheus GaugeVec格式
var (
	servicestatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "registry_service_status",
		Help: "check dubbo service registry status",
	},
	[]string{"registry_center", "application", "interface", "address", "port", "roles"},
	)
)

//主函数入口
func main() {
	//解析confpath参数
	flag.Parse()

	//从配置文件中获取信息
	zkhosts,authstr,rootpath,prometheus_port := readconf(confpath)
	//注册prometheus结构参数
	prometheus.MustRegister(servicestatus)
	//并发执行获取dubbo信息并解析
	go func(){
		for{
			fmt.Println("==========new metrics==========")
			//获取到的数据放 dubboServiceInfo 中
			dubboServiceInfo := collecter(zkhosts,authstr,rootpath)
			for i:=0;i<len(dubboServiceInfo);i++{
				//将状态值转换为float64，prometheus接受的格式
				v1, _ :=strconv.ParseFloat(dubboServiceInfo[i][6],64)
				//以label加数值的形式抛出
				servicestatus.With(prometheus.Labels{"registry_center":"dubbo", "application":dubboServiceInfo[i][0], "interface":dubboServiceInfo[i][1], "address":dubboServiceInfo[i][3], "port":dubboServiceInfo[i][4], "roles":dubboServiceInfo[i][5]}).Set(v1)
			}
			//执行完一次后，60秒后下一次执行
			time.Sleep(time.Duration(60)*time.Second)
		}
	}()

	//将prometheus数据http协议抛出
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
	        <head><title>check_exporter</title></head>
	        <body>
	        <h1>check_exporter</h1>
	        <p><a href='metrics'>Metrics</a></p>
	        </body>
	        </html>`))
	})
	//log.Fatal(http.ListenAndServe(":8080", nil))
	log.Fatal(http.ListenAndServe(":"+prometheus_port, nil))
}