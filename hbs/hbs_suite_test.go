package hbs_test
 
import (
       . "github.com/onsi/ginkgo"
       "github.com/onsi/ginkgo/reporters"
       . "github.com/onsi/gomega"
       "testing"
)
 
func TestHbs(t *testing.T) {
       RegisterFailHandler(Fail)
       junitReporter := reporters.NewJUnitReporter("hbs.junit.xml")
       RunSpecsWithDefaultAndCustomReporters(t, "hbs suite", []Reporter{junitReporter})
       //RunSpecs(t, "G Suite")
}
