package lrpstats_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/receptor/fake_receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/cc_messages"
	"github.com/cloudfoundry-incubator/tps/handler/lrpstats"
	"github.com/cloudfoundry-incubator/tps/handler/lrpstats/fakes"
	"github.com/cloudfoundry/noaa/events"
	"github.com/gogo/protobuf/proto"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("Stats", func() {
	const authorization = "something good"
	const guid = "my-guid"
	const logGuid = "log-guid"

	var (
		handler        http.Handler
		response       *httptest.ResponseRecorder
		request        *http.Request
		noaaClient     *fakes.FakeNoaaClient
		receptorClient *fake_receptor.FakeClient
		logger         *lagertest.TestLogger
	)

	BeforeEach(func() {
		var err error

		receptorClient = new(fake_receptor.FakeClient)
		noaaClient = &fakes.FakeNoaaClient{}
		logger = lagertest.NewTestLogger("test")
		handler = lrpstats.NewHandler(receptorClient, noaaClient, logger)
		response = httptest.NewRecorder()
		request, err = http.NewRequest("GET", "/v1/actual_lrps/:guid/stats", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		handler.ServeHTTP(response, request)
	})

	Describe("Validation", func() {
		It("fails with a missing authorization header", func() {
			Expect(response.Code).To(Equal(http.StatusUnauthorized))
		})

		Context("with an authorization header", func() {
			BeforeEach(func() {
				request.Header.Set("Authorization", authorization)
			})

			It("fails with no guid", func() {
				Expect(response.Code).To(Equal(http.StatusBadRequest))
			})
		})
	})

	Describe("retrieve container metrics", func() {
		BeforeEach(func() {
			request.Header.Set("Authorization", authorization)
			request.Form = url.Values{}
			request.Form.Add(":guid", guid)

			noaaClient.ContainerMetricsReturns([]*events.ContainerMetric{
				{
					ApplicationId: proto.String("appId"),
					InstanceIndex: proto.Int32(5),
					CpuPercentage: proto.Float64(4),
					MemoryBytes:   proto.Uint64(1024),
					DiskBytes:     proto.Uint64(2048),
				},
			}, nil)

			receptorClient.GetDesiredLRPReturns(receptor.DesiredLRPResponse{
				LogGuid:     logGuid,
				ProcessGuid: guid,
			}, nil)

			receptorClient.ActualLRPsByProcessGuidReturns([]receptor.ActualLRPResponse{
				{
					Index:        5,
					State:        receptor.ActualLRPStateRunning,
					Since:        124578,
					InstanceGuid: "instanceId",
					ProcessGuid:  guid,
				},
			}, nil)
		})

		It("returns a map of stats & status per index in the correct units", func() {

			expectedLRPInstance := cc_messages.LRPInstance{
				ProcessGuid:  guid,
				InstanceGuid: "instanceId",
				Index:        5,
				State:        cc_messages.LRPInstanceStateRunning,
				Since:        124578,
				Stats: &cc_messages.LRPInstanceStats{
					CpuPercentage: 0.04,
					MemoryBytes:   1024,
					DiskBytes:     2048,
				},
			}
			var stats []cc_messages.LRPInstance

			Expect(response.Code).To(Equal(http.StatusOK))
			Expect(response.Header().Get("Content-Type")).To(Equal("application/json"))
			err := json.Unmarshal(response.Body.Bytes(), &stats)
			Expect(err).NotTo(HaveOccurred())
			Expect(stats).To(ConsistOf(expectedLRPInstance))
		})

		It("calls ContainerMetrics", func() {
			Expect(noaaClient.ContainerMetricsCallCount()).To(Equal(1))
			guid, token := noaaClient.ContainerMetricsArgsForCall(0)
			Expect(guid).To(Equal(logGuid))
			Expect(token).To(Equal(authorization))
		})

		Context("when ContainerMetrics fails", func() {
			BeforeEach(func() {
				noaaClient.ContainerMetricsReturns(nil, errors.New("bad stuff happened"))
			})

			It("responds with empty stats", func() {
				expectedLRPInstance := cc_messages.LRPInstance{
					ProcessGuid:  guid,
					InstanceGuid: "instanceId",
					Index:        5,
					State:        cc_messages.LRPInstanceStateRunning,
					Since:        124578,
					Stats:        nil,
				}

				var stats []cc_messages.LRPInstance
				Expect(response.Code).To(Equal(http.StatusOK))
				Expect(response.Header().Get("Content-Type")).To(Equal("application/json"))
				err := json.Unmarshal(response.Body.Bytes(), &stats)
				Expect(err).NotTo(HaveOccurred())
				Expect(stats).To(ConsistOf(expectedLRPInstance))
			})

			It("logs the failure", func() {
				Expect(logger).To(Say("container-metrics-failed"))
			})
		})

		Context("when fetching actualLRPs fails", func() {
			BeforeEach(func() {
				receptorClient.ActualLRPsByProcessGuidReturns(nil, errors.New("bad stuff happened"))
			})

			It("responds with a 500", func() {
				Expect(response.Code).To(Equal(http.StatusInternalServerError))
			})

			It("logs the failure", func() {
				Expect(logger).To(Say("fetching-actual-lrp-info-failed"))
			})
		})

		It("calls Close", func() {
			Expect(noaaClient.CloseCallCount()).To(Equal(1))
		})

		Context("when Close fails", func() {
			BeforeEach(func() {
				noaaClient.CloseReturns(errors.New("you failed"))
			})

			It("ignores the error and returns a 200", func() {
				Expect(response.Code).To(Equal(200))
			})
		})
	})
})