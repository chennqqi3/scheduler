package g_test
 
import(
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
       . "github-beta.huawei.com/hipaas/scheduler/g"
       "github-beta.huawei.com/hipaas/common/redisoperate"
       nodeStorage "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       appStorage "github-beta.huawei.com/hipaas/common/storage/app"
       "sync"
       "time"
)
 
var _ = Describe("Nodes Test",func () {
       var (
              err error
              driver string = "redis"
              oncego sync.Once
              mynode *nodeStorage.Node
              myapp *appStorage.App
       )

checkNodeResult := func (node *nodeStorage.Node) {
              result := GetNode(node.IP)
              Expect(result.IP).To(Equal(node.IP))
              Expect(result.Region).To(Equal(node.Region))
              Expect(result.VMType).To(Equal(node.VMType))
              Expect(result.CPU).To(Equal(node.CPU))
              Expect(result.Memory).To(Equal(node.Memory))
              Expect(result.Images).To(Equal(node.Images))
              Expect(result.Version).To(Equal(node.Version))
              Expect(result.AvailableZone).To(Equal(node.AvailableZone))                
       }
 
       BeforeEach(func () {
              // first init redisConn by g.config
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
 
              // put some node to redis
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
              myapp = &appStorage.App{
                     Name: "uniTestAppNamehcy1",
                     Memory: 1024,
                     CPU: 1,
                     Instance: 2,
              }
              UpdateNode(mynode)
              // check update result
              checkNodeResult(mynode)
       })
 
       AfterEach(func () {
              // delete nodes in redis
              DeleteNode(mynode.IP)
       })
 
       Describe("UpdateNode",func () {
              var tmpnode nodeStorage.Node
 
              JustBeforeEach(func () {
                     tmpnode = *mynode                       
              })
			  AfterEach(func () {
                     DeleteNode(tmpnode.IP)
              })
 
              It("When node not exist",func () {
                     By("Step 1,UpdateNode",func () {
                            tmpnode.IP = "127.0.0.1"
                            tmpnode.Region = "unitTRegion2"
                            tmpnode.CPU = 10
                            UpdateNode(&tmpnode)
                     })    
                     By("Step 2,Check result",func () {
                            checkNodeResult(&tmpnode)
                     })
              })    
              It("when node exist",func () {
                     By("Step 1,UpdateNode",func () {
                            tmpnode.Region = "unitTRegion3"
                            tmpnode.CPU = 11
                            tmpnode.Version = "1.6.9"
                            tmpnode.AvailableZone = "testAZ2"
                            UpdateNode(&tmpnode)
                     })					 
					 By("Step 2,Check result",func () {
                            checkNodeResult(&tmpnode)    
                     })
              })
       })
       Describe("DeleteStaleNode",func () {
              It("When normal",func () {
                     result := DeleteStaleNode(time.Now().Unix() 1)
                     // todo:
                     // Expect(len(result)).NotTo(Equal(0))
                     Expect(result).Should(ContainElement(mynode.IP))
              })
       })
       Describe("DeleteNode",func () {
              It("When IP is not exist",func () {
                     notexistip := "127.0.1.1"
                     err := DeleteNode(notexistip)
                     Expect(err).To(BeNil())
              })
              It("When IP is exist",func () {
                     err := DeleteNode(mynode.IP)
                     Expect(err).To(BeNil())
              })
       })
	   Describe("TheOne",func () {
              It("when normal",func () {
                     result := TheOne()
                     Expect(result).NotTo(BeNil())
              })
       })
       Describe("GetNode",func () {
              It("When IP is not exist",func () {
                     notexistip := "127.0.1.1"
                     result := GetNode(notexistip)
                     Expect(result).To(BeNil())
              })
              It("When IP is exist",func () {
                     result := GetNode(mynode.IP)
                     Expect(result).NotTo(BeNil())
                     Expect(result.IP).To(Equal(mynode.IP))
              })
       })
       Describe("GetRegionByNode",func () {
              It("When IP is not exist",func () {
                     notexistip := "127.0.1.1"
                     result,err := GetRegionByNode(notexistip)
                     Expect(err).To(HaveOccurred())
                     Expect(result).To(Equal(""))
              })
			  It("When IP is exist",func () {
                     result,err := GetRegionByNode(mynode.IP)
                     Expect(err).To(BeNil())
                     Expect(result).To(Equal(mynode.Region))
              })
       })
       Describe("ChooseNode",func () {
              var tmpnode1 nodeStorage.Node
              var tmpnode2 nodeStorage.Node
 
              JustBeforeEach(func () {
                     tmpnode1 = *mynode
                     tmpnode1.Region = "unitTRegion4"
                     tmpnode1.IP = "10.101.10.102"
                     tmpnode2 = *mynode
                     tmpnode2.Region = "unitTRegion4"
                     tmpnode2.IP = "10.101.10.103"
 
                     UpdateNode(&tmpnode1)
                     UpdateNode(&tmpnode2)
              })
              AfterEach(func () {
                     err := DeleteNode(tmpnode1.IP)
                     Expect(err).To(BeNil())
                     DeleteNode(tmpnode2.IP)
              })
			  It("When region is not exist",func () {
                     notexistregion := "testnotexist"
                     result := ChooseNode(myapp,notexistregion,1)
                     Expect(len(result)).To(Equal(0))
              })
              It("When only one node has this region",func () {
                     By("Step 1,Memory lack",func () {
                            result := ChooseNode(myapp,mynode.Region,10)
                            Expect(len(result)).To(Equal(0))
                     })
                     By("Step 2,Memory enough",func () {
                            deployCnt := 1
                            myapp.Region = mynode.Region
                            result := ChooseNode(myapp,mynode.Region,deployCnt)
                            Expect(len(result)).To(Equal(deployCnt))
                     })
              })
              It("When region has more node",func () {
					By("Step 1,Memory lack",func () {
                            myapp.Memory = 1024 * 10
                            result := ChooseNode(myapp,tmpnode2.Region,1)
                            Expect(len(result)).To(Equal(0))
                     })
                     By("Step 2,Memory enough,but deployCnt too much",func () {
                            myapp.Memory = 1024
                            result := ChooseNode(myapp,tmpnode2.Region,10)
                            Expect(len(result)).To(Equal(0))
                     })
                     By("Step 3,Memory enough,and deployCnt equal node",func () {
                            myapp.Memory = 1024
                            deployCnt := 2
                            result := ChooseNode(myapp,tmpnode2.Region,deployCnt)
                            Expect(len(result)).To(Equal(deployCnt))
                            Expect(result[tmpnode1.IP]).To(Equal(1))
							Expect(result[tmpnode2.IP]).To(Equal(1))
                     })
                     By("Step 4,Memory enough,and deployCnt less then node",func () {
                            myapp.Memory = 1024
                            deployCnt := 1
                            result := ChooseNode(myapp,tmpnode2.Region,deployCnt)
                            Expect(len(result)).To(Equal(deployCnt))
                     })
              })
       })
       Describe("NewSelectNode",func () {
              var tmpnode1 nodeStorage.Node
              var tmpnode2 nodeStorage.Node
 
              JustBeforeEach(func () {
                     tmpnode1 = *mynode
                     tmpnode1.Region = "unitTRegion4"
                     tmpnode1.IP = "10.101.10.102"
                     tmpnode1.VMType = "container"
                     tmpnode2 = *mynode
					 tmpnode2.Region = "unitTRegion4"
                     tmpnode2.IP = "10.101.10.103"
                     tmpnode2.VMType = "container"
 
                     UpdateNode(&tmpnode1)
                     UpdateNode(&tmpnode2)
              })
              AfterEach(func () {
                     DeleteNode(tmpnode1.IP)
                     DeleteNode(tmpnode2.IP)
              })
              It("When app.CPU is 0 or app.Memory is 0",func () {
                     By("Step 1,CPU is 0",func () {
                            myapp.CPU = 0
                            _,selcount,err := NewSelectNode(myapp,mynode.Region)
                            Expect(err).To(HaveOccurred())
                            Expect(len(selcount)).To(Equal(1))
                     })
                     By("Step 2,Memory is 0",func () {
                            myapp.CPU = 4
                            myapp.Memory = 0
							_,selcount,err := NewSelectNode(myapp,mynode.Region)
                            Expect(err).To(HaveOccurred())
                            Expect(len(selcount)).To(Equal(1))
                     })
              })    
              It("When region is empty",func () {
                     myapp.VMType = mynode.VMType
                     nodeslic,selcount,err := NewSelectNode(myapp,"")
                     Expect(err).To(BeNil())  
                     Expect(len(selcount)).NotTo(Equal(0))
                     Expect(nodeslic).NotTo(BeNil())
              })
              It("When region is not empty",func () {
                     myapp.VMType = tmpnode2.VMType
                     nodeslic,selcount,err := NewSelectNode(myapp,tmpnode2.Region)
                     Expect(err).To(BeNil())  
                     Expect(len(selcount)).NotTo(Equal(0))
                     Expect(nodeslic).NotTo(BeNil())                 
              })
       })
	   Describe("SelectNode",func () {
              var tmpnode1 nodeStorage.Node
              var tmpnode2 nodeStorage.Node
              var allnode []*nodeStorage.Node
 
              JustBeforeEach(func () {
                     tmpnode1 = *mynode
                     tmpnode1.Region = "unitTRegion4"
                     tmpnode1.IP = "10.101.10.102"
                     tmpnode1.VMType = "container"
                     tmpnode2 = *mynode
                     tmpnode2.Region = "unitTRegion4"
                     tmpnode2.IP = "10.101.10.103"
                     tmpnode2.VMType = "container"
 
                     UpdateNode(&tmpnode1)
                     UpdateNode(&tmpnode2)
                     allnode = []*nodeStorage.Node{
                            mynode,
                            &tmpnode1,
                            &tmpnode2,
                     }
              })
			  AfterEach(func () {
                     DeleteNode(tmpnode1.IP)
                     DeleteNode(tmpnode2.IP)
              })
              It("When app.CPU is 0 or app.Memory is 0",func () {
                     By("Step 1,CPU is 0",func () {
                            myapp.CPU = 0
                            _,selcount,err := SelectNode(myapp,allnode,mynode.Region)
                            Expect(err).To(HaveOccurred())
                            Expect(len(selcount)).To(Equal(1))
                     })
                     By("Step 2,Memory is 0",func () {
                            myapp.CPU = 4
                            myapp.Memory = 0
                            _,selcount,err := SelectNode(myapp,allnode,mynode.Region)
                            Expect(err).To(HaveOccurred())
                            Expect(len(selcount)).To(Equal(1))
                     })
              })
			  It("When allnode is empty",func () {
                     allnode = []*nodeStorage.Node{}
                     _,selcount,err := SelectNode(myapp,allnode,mynode.Region)
                     Expect(err).To(BeNil())
                     Expect(len(selcount)).To(Equal(1))                    
              })
              It("When region is empty",func () {
                     myapp.VMType = mynode.VMType
                     nodeslic,selcount,err := SelectNode(myapp,allnode,"")
                     Expect(err).To(BeNil())  
                     Expect(len(selcount)).NotTo(Equal(0))
                     Expect(nodeslic).NotTo(BeNil())
              })
              It("When region is not empty",func () {
                     myapp.VMType = tmpnode2.VMType
                     nodeslic,selcount,err := SelectNode(myapp,allnode,tmpnode2.Region)
					 Expect(err).To(BeNil())  
                     Expect(len(selcount)).NotTo(Equal(0))
                     Expect(nodeslic).NotTo(BeNil())                 
              })           
       })
       Describe("NewChooseNode",func () {
              var tmpnode1 nodeStorage.Node
              var tmpnode2 nodeStorage.Node
              var nodeslic *nodeStorage.NodeSelectSlice
 
              JustBeforeEach(func () {
                     tmpnode1 = *mynode
                     tmpnode1.Region = "unitTRegion4"
                     tmpnode1.IP = "10.101.10.102"
                     tmpnode1.VMType = "container"
                     tmpnode2 = *mynode
                     tmpnode2.Region = "unitTRegion4"
                     tmpnode2.IP = "10.101.10.103"
                     tmpnode2.VMType = "container"
					 UpdateNode(&tmpnode1)
                     UpdateNode(&tmpnode2)
                     nodeslic = &nodeStorage.NodeSelectSlice{
                            &nodeStorage.NodeSelect {
                                   Node: mynode,
                                   Count: 1,
                            },
                            &nodeStorage.NodeSelect {
                                   Node: &tmpnode1,
                                   Count: 2,
                            },
                            &nodeStorage.NodeSelect {
                                   Node: &tmpnode2,
                                   Count: 0,
                            },                         
                     }
              })
              AfterEach(func () {
                     err := DeleteNode(tmpnode1.IP)
                     Expect(err).To(BeNil())
                     DeleteNode(tmpnode2.IP)
              })
			  It("When nodeslic is empty",func () {
                     nodeslic = &nodeStorage.NodeSelectSlice{}
                     result := NewChooseNode(nodeslic,myapp,1)
                     Expect(len(result)).To(Equal(0))
              })    
              It("When select failed",func () {
                     tmpslic := &nodeStorage.NodeSelectSlice{
                            &nodeStorage.NodeSelect{
                                   Node: mynode,
                                   Count: 0,
                            },
                            &nodeStorage.NodeSelect {
                                   Node: &tmpnode1,
                                   Count: 0,
                            },
                            &nodeStorage.NodeSelect {
                                   Node: &tmpnode2,
                                   Count: 0,
                            },                                
                     }
					 result := NewChooseNode(tmpslic,myapp,3)
                     Expect(len(result)).To(Equal(0))
              })
              It("When select success",func () {
                     result := NewChooseNode(nodeslic,myapp,3)
                     Expect(len(result)).To(Equal(2))
              })
       })
})
