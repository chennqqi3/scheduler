package main

import (
	"flag"
	"fmt"
	"os"

	"github-beta.huawei.com/hipaas/glog"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	"github-beta.huawei.com/hipaas/common/appevent"
	ME "github-beta.huawei.com/hipaas/common/msgengine"
	"github-beta.huawei.com/hipaas/common/redisoperate"
	"github-beta.huawei.com/hipaas/common/storage/node"
	"github-beta.huawei.com/hipaas/common/storage/realstate"
	"github-beta.huawei.com/hipaas/scheduler/cron/sync"
	"github-beta.huawei.com/hipaas/scheduler/executor"
	"github-beta.huawei.com/hipaas/scheduler/fastsetting"
	"github-beta.huawei.com/hipaas/scheduler/g"
	"github-beta.huawei.com/hipaas/scheduler/hbs"
	"github-beta.huawei.com/hipaas/scheduler/http"
	"github-beta.huawei.com/hipaas/scheduler/plugin/algorithm/factory"
	_ "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm/provider"
	"github-beta.huawei.com/hipaas/scheduler/router"
)

func main() {
	cfg := flag.String("c", "", "configuration file")
	version := flag.Bool("version", false, "show version")
	g.FlagInit()
	flag.Parse()
	defer glog.Flush()

	if flag.NArg() != 0 || (flag.NFlag() == 0 && flag.NArg() == 0) {
		flag.PrintDefaults()
		os.Exit(0)
	}

	ShowVersion()
	if *version {
		os.Exit(0)
	}

	err := g.ParseConfig(*cfg)
	if err != nil {
		glog.Fatalf("g.ParseConfig fail: %v", err)
	}

	g.RedisConnPool, err = redisoperate.InitRedisConnPool(g.Config().Redis)
	if nil != err {
		glog.Fatalf(`InitRedisConnPool fail, err: %v`, err)
	}
	_, err = g.RedisConnPool.Dial()
	if err != nil {
		glog.Fatalf(`InitRedisConnPool fail, object: "g.RedisConnPool", err: %v`, err)
	}

	if err = g.NewDbMgr(); nil != err {
		glog.Fatalf(`Init mysql conn pool fail, err: %v`, err)
	}

	driver := "redis"
	g.RealState, err = realstate.NewSafeRealState(driver, g.RedisConnPool)
	if err != nil {
		glog.Fatalf("RealState can not init, Error: %v", err)
	}

	g.RealNodeState, err = node.NewSafeNodeState(driver, g.RedisConnPool)
	if err != nil {
		glog.Fatalf("RealNodeState can not init! Error: %v", err)
	}

	g.RealMinionState, err = realstate.NewSafeRealState(driver, g.RedisConnPool)
	if err != nil {
		glog.Fatal("RealMinionState can not init, Error: %v", err)
	}
	if err := g.NewNetClient(); nil != err {
		glog.Fatalf("new netservice failed. Error: %v", err)
	}
	fastsetting.NewFastsetting(g.RedisConnPool, g.RealState)
	// create and init dockerManager
	executor.GetDockerManager()
	dockStatus := realstate.DockerStatusNew(g.RedisConnPool)
	if dockStatus == nil {
		glog.Fatal("executor.DockerStatusNew() error!")
	}
	err = dockStatus.DelAllCreatingStatus()
	if err != nil {
		glog.Fatalf("docker status delete all creating status fail, err: %v", err)
	}

	if err = ME.StartMsgEngine(g.RedisConnPool, &g.MsgEngineStateChan, g.Config().LocalIP); nil != err {
		glog.Fatal(err)
	}
	if err = g.StartService(); nil != err {
		glog.Fatal(err)
	}
	if err = g.StartCtnrPublisher(); nil != err {
		glog.Fatal(err)
	}
	if err = router.StartRouterDrv(); nil != err {
		glog.Fatal(err)
	}
	if err := appevent.StartAppEventEngine(g.RedisConnPool); nil != err {
		glog.Fatal(err)
	}

	factoryConfig := factory.NewConfigFactory(g.RealNodeState.GetAllNode, dockStatus.DockerStatusGetAllNodes, g.RealState.Containers, dockStatus.DockerStatusUpdate)
	algoConfig, err := factoryConfig.Create()
	if err != nil {
		glog.Fatalf("factoryConfig.Create error! %v", err)
	}

	compare := sync.NewSyncState(dockStatus, g.Config, algoConfig)
	go compare.Sync(g.Config().Interval)
	go compare.RequireResource(g.Config().Interval)
	go compare.HealthCheck(g.Config().Health.Interval)
	go compare.CheckStale(3 * g.Config().Interval)
	go hbs.Start()
	go g.WatchFile(*cfg)

	handler := http.New()
	addr := fmt.Sprintf("%s:%d", g.Config().HTTP.Addr, g.Config().HTTP.Port)
	members := grouper.Members{
		{"server", http_server.New(addr, handler)},
	}
	group := grouper.NewOrdered(os.Interrupt, members)
	monitor := ifrit.Invoke(sigmon.New(group))

	err = <-monitor.Wait()
	if err != nil {
		glog.Fatalf("exited-with-failure: %v", err)
	}
}
