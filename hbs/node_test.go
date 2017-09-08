package hbs_test
 
import (
       storagenode "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/common/storage/createstate"
       // "github-beta.huawei.com/hipaas/monitor"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "github-beta.huawei.com/hipaas/scheduler/hbs"
       "github-beta.huawei.com/hipaas/common/redisoperate"
       "github-beta.huawei.com/hipaas/common/crypto/aes"
 
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
 
       "fmt"
       "time"
       "strconv"
       "math/rand"
)
 
const testcfg = "../cfg.example.json"
 
var _ = Describe("hbs", func() {
       var (
              req  *storagenode.NodeRequest
              resp *storagenode.NodeResponse
              node storagenode.Node
       )
 
       defer GinkgoRecover()
 
       randomStr := func(prefix string, ls int) string{
              str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
              bytes := []byte(str)
              result := []byte{}
              r := rand.New(rand.NewSource(time.Now().UnixNano()))
              for i :=0;i < ls; i++ {
                     result = append(result,bytes[r.Intn(len(bytes))])
              }
              return prefix + string(result)
       }
 
       nodestate := new(hbs.NodeState)
       g.FlagInit()
 
       BeforeEach(func() {
              err := g.ParseConfig(testcfg)
              if err != nil {
                     Fail("g.ParseConfig fail: "+err.Error())
              }
 
              g.RedisConnPool,err = redisoperate.InitRedisConnPool(g.Config().Redis)
              if err != nil {
                     Fail("g.RedisConnPool InitRedisConnPool fail: "+err.Error())
              }
 
 
              if err = g.NewDbMgr(); nil != err {
                     Fail("Init mysql conn pool fail: "+err.Error())
              }
 
              driver := "redis"
              g.RealState,err  = realstate.NewSafeRealState(driver, g.RedisConnPool)
              if err != nil {
                     Fail("RealState init failed")
              }
 
              g.RealNodeState,err = storagenode.NewSafeNodeState(driver, g.RedisConnPool)
              if err != nil {
                     Fail("RealNodeState init failed")
              }
 
              g.RealMinionState,err = realstate.NewSafeRealState(driver, g.RedisConnPool)
              if err != nil {
                     Fail("RealMinionState init failed")
              }
              node = storagenode.Node{
                     IP:           "10.42.63.33",
                     Region:       "regiontest",
                     VMType:       "hipaas",
                     CPU:          2,
                     Memory:       512,
                     CPUVirtUsage: 0,
                     MemVirtUsage: 256,
                     MemFree:      512,
                     UpdateAt:     1000,
              }
              req = &storagenode.NodeRequest{
                     Node: node,
                     Containers: []*realstate.Container{
                            &realstate.Container{
                                   IP:          "10.32.52.62",
                                   Image:       "/dfjds.image",
                                   Recovery:    true,
                                   AppName:     "jetty",
                                   Ports:       []*realstate.Port{&realstate.Port{PublicPort: 8080}},
                                   Status:      "1",
                                   Hostname:    "zhangmanjuan",
                                   ContainerIP: "10.31.42.53",
                                   Region:      "df",
                                   VMType:      "hipaas",
                                   CPU:         2,
                                   Memory:      512,
                                   Sign:        "zhangmanjuan",
                            },
                     },
                     Sign: "cuvEg9NGVj15MsFBSuE2ng==",
              }
 
              resp = &storagenode.NodeResponse{
                     Code: 200,
              }
       })
 
       AfterEach(func() {
              conn := g.RedisConnPool.Get()
              conn.Do("DEL", "hipaas_app_jetty")
              conn.Close()
       })
 
       Context("Push", func() {
              It("when req is nil", func() {
                     var req1 *storagenode.NodeRequest
                     err := nodestate.Push(req1, resp)
                     Expect(err).To(HaveOccurred(), "NodeRequest can't be nil")
                     Expect(resp.Code).To(Equal(1))
              })
              It("req.Containers is null", func() {
                     req1 := &storagenode.NodeRequest{
                            Node: node,
                            Sign: "cuvEg9NGVj15MsFBSuE2ng==",
                     }
                     err := nodestate.Push(req1, resp)
                     Expect(err).To(BeNil())
                     Expect(req1.Node.UpdateAt).To(Equal(time.Now().Unix()))
              })
              It("when container not null", func() {
                     err := nodestate.Push(req, resp)
                     Expect(err).To(BeNil())
                     Expect(req.Containers[0].UpdateAt).To(Equal(time.Now().Unix()))
              })
       })
       Context("NodeDown", func() {
              It("when ip is null", func() {
                     ip := ""
                     err := nodestate.NodeDown(ip, resp)
                     Expect(err).To(HaveOccurred(), "ip can't be empty")
                     Expect(resp.Code).To(Equal(1))
              })
              It("when ip is not exist", func() {
                     ip := "0.0.0.0"
                     err := nodestate.NodeDown(ip, resp)
                     Expect(err).To(BeNil())
                     Expect(resp.Code).To(Equal(200))
              })
              It("when ip is exist", func() {
                     ip := node.IP
                     err := nodestate.NodeDown(ip, resp)
                     Expect(err).To(BeNil())
                     Expect(g.RealState.ContainerCount(req.Containers[0].AppName)).To(Equal(0))
              })
       })
       Context("Heartbeat",func(){
              It("when req is nil",func(){
                     var req1 *storagenode.NodeHeartbeat
                     err := nodestate.Heartbeat(req1,resp)
                     Expect(err).To(HaveOccurred(),"node.NodeRequest can't be nil")
                     Expect(resp.Code).To(Equal(1))
              })
              It("when SignMsg.Switch is true",func(){
                     By("Sign more than interval",func(){
                            now_t := time.Now().Unix()
                            tmpSign := strconv.Itoa(int(now_t - g.Config().SignMsg.Interval - 1))
                            myaes,err := aes.New("")
                            Expect(err).NotTo(HaveOccurred())
                            sign,err := myaes.Encrypt(tmpSign)
                            Expect(err).NotTo(HaveOccurred())
                            req1 := &storagenode.NodeHeartbeat {
                                   NodeIP: "10.30.50.60",
                                   Sign: sign,
                            }                   
                            err = nodestate.Heartbeat(req1,resp)
                            Expect(err).To(HaveOccurred(),"request refuse!")
                     })
                     By("Sign less than interval",func(){
                            now_t := time.Now().Unix()
                            tmpSign := strconv.Itoa(int(now_t))
                            myaes,err := aes.New("")
                            Expect(err).NotTo(HaveOccurred())
                            sign,err := myaes.Encrypt(tmpSign)
                            Expect(err).NotTo(HaveOccurred())
                            req1 := &storagenode.NodeHeartbeat {
                                   NodeIP: "10.30.50.60",
                                   Sign: sign,
                            }                   
                            err = nodestate.Heartbeat(req1,resp)
                            Expect(err).To(BeNil())
                            Expect(resp.Code).To(Equal(200))
                     })
              })
              It("when SignMsg.Switch is false",func(){
                     if g.Config().SignMsg.Switch != true {
                            req1 := &storagenode.NodeHeartbeat {
                                   NodeIP: "10.30.50.60",
                            }
                            err := nodestate.Heartbeat(req1,resp)
                            Expect(err).To(BeNil())
                            Expect(resp.Code).To(Equal(200))
                     }
              })    
       })
       Context("AppTaskResult",func(){
              var tmpname string
 
              JustBeforeEach(func () {
                     tmpname = randomStr("hcytest-",10)
              })
              AfterEach(func () {
                     rc := g.RedisConnPool.Get()
                     tmpkey := fmt.Sprintf("hipaas_app_%s",tmpname)
                     rc.Do("DEL",tmpkey)
                     rc.Close()
              })
 
              It("when fail is 0",func(){
                     req1 := &createstate.AppTaskCreateCtnrResult {
                            AppName: tmpname,
                            Fail: []string{},
                            Success: []*realstate.Container {
                                   &realstate.Container{
                                          ID: randomStr("hcytestid-",14),
                                          IP: "10.31.55.13",
                                          AppName: tmpname,
                                          Status: "Up",
                                   },
                                   &realstate.Container{
                                          ID: randomStr("htestid-",14),
                                          IP: "10.39.54.47",
                                          AppName: tmpname,
                                          Status: "Up",
                                   },
                            },
                     }
                     resp1 := &createstate.RPCCommonRespone {}
                     err := nodestate.AppTaskResult(req1,resp1)
                     Expect(err).To(BeNil())
              })
              It("when fail is not 0",func(){
                     req1 := &createstate.AppTaskCreateCtnrResult {
                            AppName: tmpname,
                            Fail: []string{"create container failed"},
                            Success: []*realstate.Container {
                                   &realstate.Container{
                                          ID: randomStr("hcytestid-",14),
                                          IP: "10.66.27.39",
                                          AppName: tmpname,
                                          Status: "Up",
                                          Hostname: "23lsdfj23df",
                                   },
                            },
                     }
                     resp1 := &createstate.RPCCommonRespone {}
                     err := nodestate.AppTaskResult(req1,resp1)
                     Expect(err).To(BeNil())
              })
       })
})
