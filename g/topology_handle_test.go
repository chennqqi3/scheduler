package g_test
 
import (
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
       . "github-beta.huawei.com/hipaas/scheduler/g"
       "github-beta.huawei.com/hipaas/common/redisoperate"
       nodeStorage "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "sync"
       "time"
)
 
// GetGlobalTopology() (*model.GlobalLevel, error)
var _ = Describe("Topology_handle Test",func () {
       var (
              err error
              driver string = "redis"
              oncego sync.Once
              mynode *nodeStorage.Node
       )

BeforeEach(func () {
              oncego.Do(func () {
                     // Config init
                     FlagInit()
                     err = ParseConfig("../cfg.example.json")
                     Expect(err).NotTo(HaveOccurred())
 
                     // Redis pool init
                     RedisConnPool, err = redisoperate.InitRedisConnPool(Config().Redis)
                     Expect(err).NotTo(HaveOccurred())
 
                     RealNodeState,err = nodeStorage.NewSafeNodeState(driver, RedisConnPool)
                     Expect(err).NotTo(HaveOccurred())
 
                     RealState, err = realstate.NewSafeRealState(driver, RedisConnPool)
                     Expect(err).NotTo(HaveOccurred())    
              })
			  mynode = &nodeStorage.Node{
                     // Default this IP node is not exist
                     IP: "10.101.10.101",
                     Region: "unitTRegion1",
                     VMType: "was",
                     CPU: 8,
                     Memory: 1024*8,
                     CPUVirtUsage: 8,
                     MemVirtUsage: 1024*8,
                     CPUUsage: 4,
                     MemUsage: 1024*4,
                     MemFree: 1024*3,
                     UpdateAt: time.Now().Unix(),
                     HeartBeatAt: time.Now().Unix(),
                     Images: []string{"paas-dockerhub-beta.huawei.com:5000/test/unitTestimg:latest"},
                     Fails: 0,
                     AvailableZone: "testAZ1",
                     Version: "1.6.8",
              }
       })
       AfterEach(func () {
 
       })
	   Describe("GetGlobalTopology",func () {
              var (
                     tmpnode1 nodeStorage.Node
                     tmpnode2 nodeStorage.Node
                     tmpnode3 nodeStorage.Node
              )
 
              JustBeforeEach(func () {
                     tmpnode1 = *mynode                     
                     tmpnode2 = *mynode
                     tmpnode3 = *mynode
 
                     //
                     tmpnode1.IP = "10.101.10.102"
                     tmpnode1.AvailableZone = "testAZ1"
                     tmpnode2.IP = "10.101.10.103"
                     tmpnode2.AvailableZone = "testAZ2"
                     tmpnode3.IP = "10.101.10.104"
                     tmpnode3.AvailableZone = "testAZ3"
                                                              
                     UpdateNode(&tmpnode1)
                     UpdateNode(&tmpnode2)
                     UpdateNode(&tmpnode3)
              })
			  AfterEach(func () {
                     err := DeleteNode(tmpnode1.IP)
                     Expect(err).To(BeNil())
                     DeleteNode(tmpnode2.IP)
                     DeleteNode(tmpnode3.IP)
              })
              It("Testcase",func () {
                     mytop := &Topology{}
                     result,err := mytop.GetGlobalTopology()
                     Expect(err).To(BeNil())
                     Expect(result).NotTo(BeNil())
              })
       })
})

