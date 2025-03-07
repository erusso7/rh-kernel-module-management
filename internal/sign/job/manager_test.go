package signjob

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kmmv1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/api"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/auth"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/constants"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/registry"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/utils"
)

var _ = Describe("ShouldSync", func() {
	var (
		ctrl        *gomock.Controller
		authFactory *auth.MockRegistryAuthGetterFactory
		reg         *registry.MockRegistry
		mgr         *signJobManager
	)

	const (
		moduleName    = "module-name"
		imageName     = "image-name"
		namespace     = "some-namespace"
		kernelVersion = "1.2.3"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		authFactory = auth.NewMockRegistryAuthGetterFactory(ctrl)
		reg = registry.NewMockRegistry(ctrl)
		mgr = NewSignJobManager(nil, nil, authFactory, reg)
	})

	It("should return false if there was not sign section", func() {
		ctx := context.Background()

		mld := &api.ModuleLoaderData{}
		shouldSync, err := mgr.ShouldSync(ctx, mld)

		Expect(err).ToNot(HaveOccurred())
		Expect(shouldSync).To(BeFalse())
	})

	It("should return false if image already exists", func() {
		ctx := context.Background()

		mld := &api.ModuleLoaderData{
			Name:            moduleName,
			Namespace:       namespace,
			ImageRepoSecret: &v1.LocalObjectReference{Name: "pull-push-secret"},
			ContainerImage:  imageName,
			Sign:            &kmmv1beta1.Sign{},
		}

		gomock.InOrder(
			authFactory.EXPECT().NewRegistryAuthGetterFrom(mld),
			reg.EXPECT().ImageExists(ctx, imageName, nil, gomock.Any()).Return(true, nil),
		)

		shouldSync, err := mgr.ShouldSync(ctx, mld)

		Expect(err).ToNot(HaveOccurred())
		Expect(shouldSync).To(BeFalse())
	})

	It("should return false and an error if image check fails", func() {
		ctx := context.Background()

		mld := &api.ModuleLoaderData{
			Name:            moduleName,
			Namespace:       namespace,
			ImageRepoSecret: &v1.LocalObjectReference{Name: "pull-push-secret"},
			ContainerImage:  imageName,
			Sign:            &kmmv1beta1.Sign{},
		}

		gomock.InOrder(
			authFactory.EXPECT().NewRegistryAuthGetterFrom(mld),
			reg.EXPECT().ImageExists(ctx, imageName, nil, gomock.Any()).Return(false, errors.New("generic-registry-error")),
		)

		shouldSync, err := mgr.ShouldSync(ctx, mld)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("generic-registry-error"))
		Expect(shouldSync).To(BeFalse())
	})

	It("should return true if image does not exist", func() {
		ctx := context.Background()

		mld := &api.ModuleLoaderData{
			Name:            moduleName,
			Namespace:       namespace,
			ImageRepoSecret: &v1.LocalObjectReference{Name: "pull-push-secret"},
			ContainerImage:  imageName,
			Sign:            &kmmv1beta1.Sign{},
		}

		gomock.InOrder(
			authFactory.EXPECT().NewRegistryAuthGetterFrom(mld),
			reg.EXPECT().ImageExists(ctx, imageName, nil, gomock.Any()).Return(false, nil),
		)

		shouldSync, err := mgr.ShouldSync(ctx, mld)

		Expect(err).ToNot(HaveOccurred())
		Expect(shouldSync).To(BeTrue())
	})
})

var _ = Describe("Sync", func() {
	var (
		ctrl      *gomock.Controller
		maker     *MockSigner
		jobhelper *utils.MockJobHelper
		mgr       *signJobManager
	)

	const (
		imageName         = "image-name"
		previousImageName = "previous-image"
		namespace         = "some-namespace"

		moduleName    = "module-name"
		kernelVersion = "1.2.3"
		jobName       = "some-job"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		maker = NewMockSigner(ctrl)
		jobhelper = utils.NewMockJobHelper(ctrl)
		mgr = NewSignJobManager(maker, jobhelper, nil, nil)
	})

	labels := map[string]string{"kmm.node.kubernetes.io/job-type": "sign",
		"kmm.node.kubernetes.io/module.name":   moduleName,
		"kmm.node.kubernetes.io/target-kernel": kernelVersion,
	}

	mld := &api.ModuleLoaderData{
		Name:           moduleName,
		ContainerImage: imageName,
		Build:          &kmmv1beta1.Build{},
		Owner:          &kmmv1beta1.Module{},
		KernelVersion:  kernelVersion,
	}

	DescribeTable("should return the correct status depending on the job status",
		func(s batchv1.JobStatus, jobStatus utils.Status, expectsErr bool) {
			j := batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"kmm.node.kubernetes.io/job-type": "sign",
						"kmm.node.kubernetes.io/module.name":   moduleName,
						"kmm.node.kubernetes.io/target-kernel": kernelVersion,
					},
					Namespace:   namespace,
					Annotations: map[string]string{constants.JobHashAnnotation: "some hash"},
				},
				Status: s,
			}
			newJob := batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"kmm.node.kubernetes.io/job-type": "sign",
						"kmm.node.kubernetes.io/module.name":   moduleName,
						"kmm.node.kubernetes.io/target-kernel": kernelVersion,
					},
					Namespace:   namespace,
					Annotations: map[string]string{constants.JobHashAnnotation: "some hash"},
				},
				Status: s,
			}
			ctx := context.Background()

			var joberr error
			if expectsErr {
				joberr = errors.New("random error")
			}

			gomock.InOrder(
				jobhelper.EXPECT().JobLabels(mld.Name, kernelVersion, "sign").Return(labels),
				maker.EXPECT().MakeJobTemplate(ctx, mld, labels, previousImageName, true, mld.Owner).Return(&j, nil),
				jobhelper.EXPECT().GetModuleJobByKernel(ctx, mld.Name, mld.Namespace, kernelVersion, utils.JobTypeSign, mld.Owner).Return(&newJob, nil),
				jobhelper.EXPECT().IsJobChanged(&j, &newJob).Return(false, nil),
				jobhelper.EXPECT().GetJobStatus(&newJob).Return(jobStatus, joberr),
			)

			res, err := mgr.Sync(ctx, mld, previousImageName, true, mld.Owner)

			if expectsErr {
				Expect(err).To(HaveOccurred())
				return
			}

			Expect(res).To(Equal(jobStatus))
		},
		Entry("active", batchv1.JobStatus{Active: 1}, utils.Status(utils.StatusInProgress), false),
		Entry("active", batchv1.JobStatus{Active: 1}, utils.Status(utils.StatusInProgress), false),
		Entry("succeeded", batchv1.JobStatus{Succeeded: 1}, utils.Status(utils.StatusCompleted), false),
		Entry("failed", batchv1.JobStatus{Failed: 1}, utils.Status(""), true),
	)

	It("should return an error if there was an error creating the job template", func() {
		ctx := context.Background()

		gomock.InOrder(
			jobhelper.EXPECT().JobLabels(mld.Name, kernelVersion, "sign").Return(labels),
			maker.EXPECT().MakeJobTemplate(ctx, mld, labels, previousImageName, true, mld.Owner).
				Return(nil, errors.New("random error")),
		)

		Expect(
			mgr.Sync(ctx, mld, previousImageName, true, mld.Owner),
		).Error().To(
			HaveOccurred(),
		)
	})

	It("should return an error if there was an error getting running jobs", func() {
		ctx := context.Background()
		j := batchv1.Job{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "batch/v1",
				Kind:       "Job",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: namespace,
			},
		}

		gomock.InOrder(
			jobhelper.EXPECT().JobLabels(mld.Name, kernelVersion, "sign").Return(labels),
			maker.EXPECT().MakeJobTemplate(ctx, mld, labels, previousImageName, true, mld.Owner).Return(&j, nil),
			jobhelper.EXPECT().GetModuleJobByKernel(ctx, mld.Name, mld.Namespace, kernelVersion, utils.JobTypeSign, mld.Owner).Return(nil, errors.New("random error")),
		)

		Expect(
			mgr.Sync(ctx, mld, previousImageName, true, mld.Owner),
		).Error().To(
			HaveOccurred(),
		)
	})

	It("should return an error if there was an error creating the job", func() {
		ctx := context.Background()
		j := batchv1.Job{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "batch/v1",
				Kind:       "Job",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: namespace,
			},
		}

		gomock.InOrder(
			jobhelper.EXPECT().JobLabels(mld.Name, kernelVersion, "sign").Return(labels),
			maker.EXPECT().MakeJobTemplate(ctx, mld, labels, previousImageName, true, mld.Owner).Return(&j, nil),
			jobhelper.EXPECT().GetModuleJobByKernel(ctx, mld.Name, mld.Namespace, kernelVersion, utils.JobTypeSign, mld.Owner).Return(nil, utils.ErrNoMatchingJob),
			jobhelper.EXPECT().CreateJob(ctx, &j).Return(errors.New("unable to create job")),
		)

		Expect(
			mgr.Sync(ctx, mld, previousImageName, true, mld.Owner),
		).Error().To(
			HaveOccurred(),
		)
	})

	It("should create the job and return without error if it doesn't exist", func() {
		ctx := context.Background()

		j := batchv1.Job{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "batch/v1",
				Kind:       "Job",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: namespace,
			},
		}

		gomock.InOrder(
			jobhelper.EXPECT().JobLabels(mld.Name, kernelVersion, "sign").Return(labels),
			maker.EXPECT().MakeJobTemplate(ctx, mld, labels, previousImageName, true, mld.Owner).Return(&j, nil),
			jobhelper.EXPECT().GetModuleJobByKernel(ctx, mld.Name, mld.Namespace, kernelVersion, utils.JobTypeSign, mld.Owner).Return(nil, utils.ErrNoMatchingJob),
			jobhelper.EXPECT().CreateJob(ctx, &j).Return(nil),
		)

		Expect(
			mgr.Sync(ctx, mld, previousImageName, true, mld.Owner),
		).To(
			Equal(utils.Status(utils.StatusCreated)),
		)
	})

	It("should delete the job if it was edited", func() {
		ctx := context.Background()

		newJob := batchv1.Job{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "batch/v1",
				Kind:       "Job",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        jobName,
				Namespace:   namespace,
				Annotations: map[string]string{constants.JobHashAnnotation: "new hash"},
			},
		}

		gomock.InOrder(
			jobhelper.EXPECT().JobLabels(mld.Name, kernelVersion, "sign").Return(labels),
			maker.EXPECT().MakeJobTemplate(ctx, mld, labels, previousImageName, true, mld.Owner).Return(&newJob, nil),
			jobhelper.EXPECT().GetModuleJobByKernel(ctx, mld.Name, mld.Namespace, kernelVersion, utils.JobTypeSign, mld.Owner).Return(&newJob, nil),
			jobhelper.EXPECT().IsJobChanged(&newJob, &newJob).Return(true, nil),
			jobhelper.EXPECT().DeleteJob(ctx, &newJob).Return(nil),
		)

		Expect(
			mgr.Sync(ctx, mld, previousImageName, true, mld.Owner),
		).To(
			Equal(utils.Status(utils.StatusInProgress)),
		)
	})
})

var _ = Describe("GarbageCollect", func() {
	var (
		ctrl      *gomock.Controller
		jobhelper *utils.MockJobHelper
		mgr       *signJobManager
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		jobhelper = utils.NewMockJobHelper(ctrl)
		mgr = NewSignJobManager(nil, jobhelper, nil, nil)
	})

	mld := api.ModuleLoaderData{
		Name:  "moduleName",
		Owner: &kmmv1beta1.Module{},
	}

	DescribeTable("should return the correct error and names of the collected jobs",
		func(jobStatus1 batchv1.JobStatus, jobStatus2 batchv1.JobStatus, expectsErr bool) {
			job1 := batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "jobName1",
				},
				Status: jobStatus1,
			}
			job2 := batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: "jobName2",
				},
				Status: jobStatus2,
			}
			expectedNames := []string{}
			if !expectsErr {
				if job1.Status.Succeeded == 1 {
					expectedNames = append(expectedNames, "jobName1")
				}
				if job2.Status.Succeeded == 1 {
					expectedNames = append(expectedNames, "jobName2")
				}
			}
			returnedError := fmt.Errorf("some error")
			if !expectsErr {
				returnedError = nil
			}

			jobhelper.EXPECT().GetModuleJobs(context.Background(), mld.Name, mld.Namespace, utils.JobTypeSign, mld.Owner).Return([]batchv1.Job{job1, job2}, returnedError)
			if !expectsErr {
				if job1.Status.Succeeded == 1 {
					jobhelper.EXPECT().DeleteJob(context.Background(), &job1).Return(nil)
				}
				if job2.Status.Succeeded == 1 {
					jobhelper.EXPECT().DeleteJob(context.Background(), &job2).Return(nil)
				}
			}

			names, err := mgr.GarbageCollect(context.Background(), mld.Name, mld.Namespace, mld.Owner)

			if expectsErr {
				Expect(err).To(HaveOccurred())
				Expect(names).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(expectedNames).To(Equal(names))
			}
		},
		Entry("all jobs succeeded", batchv1.JobStatus{Succeeded: 1}, batchv1.JobStatus{Succeeded: 1}, false),
		Entry("1 job succeeded", batchv1.JobStatus{Succeeded: 1}, batchv1.JobStatus{Succeeded: 0}, false),
		Entry("0 job succeeded", batchv1.JobStatus{Succeeded: 0}, batchv1.JobStatus{Succeeded: 0}, false),
		Entry("error occured", batchv1.JobStatus{Succeeded: 0}, batchv1.JobStatus{Succeeded: 0}, true),
	)
})
