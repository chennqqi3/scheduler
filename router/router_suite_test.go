package router_test
 
import (
       . "github.com/onsi/ginkgo"
       "github.com/onsi/ginkgo/reporters"
       . "github.com/onsi/gomega"
       "testing"
)
 
func TestHttp(t *testing.T) {
       RegisterFailHandler(Fail)
       junitReporter := reporters.NewJUnitReporter("router.junit.xml")
       RunSpecsWithDefaultAndCustomReporters(t, "router suite", []Reporter{junitReporter})
       //RunSpecs(t, "Http Suite")
}
