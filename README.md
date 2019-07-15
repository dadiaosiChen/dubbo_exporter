# dubbo_exporter

prometheus监控架构下，监控dubbo服务中心服务状态，，目前仅监控zk为注册中心的生产者provider，若要监控消费者，可以将path参数改为consumers
config.ini文件为注册中心zk登录信息，目前zk认证仅支持digest方式，信息参考配置文件样例
执行参数可用 --help查询：
-confpath string
        The config filepath (default "config.ini")

prometheus抛出信息为
registry_service_status{address="xxxxxx",application="xxxxxx",interface="xxxxxx",port="xxxxxx",registry_center="dubbo",roles="providers"} 1
正常为1,异常（主机禁用，半权）为2。若生产者down掉会脱离zk，信息缺失，所以为0的情况，为后续修改版本。