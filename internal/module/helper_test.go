package module

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gomock "github.com/golang/mock/gomock"
	v1 "k8s.io/api/core/v1"

	"github.com/rh-ecosystem-edge/kernel-module-management/internal/api"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/auth"
	"github.com/rh-ecosystem-edge/kernel-module-management/internal/registry"
)

var _ = Describe("AppendToTag", func() {
	It("should append a tag to the image name", func() {
		name := "some-container-image-name"
		tag := "a-kmm-tag"

		Expect(
			AppendToTag(name, tag),
		).To(
			Equal(name + ":" + tag),
		)
	})

	It("should add a suffix to the already present tag", func() {
		name := "some-container-image-name:with-a-tag"
		tag := "a-kmm-tag-suffix"

		Expect(
			AppendToTag(name, tag),
		).To(
			Equal(name + "_" + tag),
		)
	})
})

var _ = Describe("IntermediateImageName", func() {
	It("should add the kmm_unsigned suffix to the target image name", func() {
		Expect(
			IntermediateImageName("module-name", "test-namespace", "some-image-name"),
		).To(
			Equal("some-image-name:test-namespace_module-name_kmm_unsigned"),
		)
	})
})

var _ = Describe("ImageExists", func() {
	const (
		imageName = "image-name"
		namespace = "test"
	)

	var (
		ctrl *gomock.Controller

		mockAuthFactory *auth.MockRegistryAuthGetterFactory
		mockRegistry    *registry.MockRegistry

		mld api.ModuleLoaderData
		ctx context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		mockAuthFactory = auth.NewMockRegistryAuthGetterFactory(ctrl)
		mockRegistry = registry.NewMockRegistry(ctrl)

		mld = api.ModuleLoaderData{}
		ctx = context.Background()
	})

	It("should return true if the image exists", func() {
		gomock.InOrder(
			mockAuthFactory.EXPECT().NewRegistryAuthGetterFrom(&mld),
			mockRegistry.EXPECT().ImageExists(ctx, imageName, gomock.Any(), nil).Return(true, nil),
		)

		exists, err := ImageExists(ctx, mockAuthFactory, mockRegistry, &mld, imageName)

		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("should return false if the image does not exist", func() {
		gomock.InOrder(
			mockAuthFactory.EXPECT().NewRegistryAuthGetterFrom(&mld),
			mockRegistry.EXPECT().ImageExists(ctx, imageName, gomock.Any(), nil).Return(false, nil),
		)

		exists, err := ImageExists(ctx, mockAuthFactory, mockRegistry, &mld, imageName)

		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

	It("should return an error if the registry call fails", func() {
		gomock.InOrder(
			mockAuthFactory.EXPECT().NewRegistryAuthGetterFrom(&mld),
			mockRegistry.EXPECT().ImageExists(ctx, imageName, gomock.Any(), nil).Return(false, errors.New("some-error")),
		)

		exists, err := ImageExists(ctx, mockAuthFactory, mockRegistry, &mld, imageName)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("some-error"))
		Expect(exists).To(BeFalse())
	})

	It("should use the ImageRepoSecret if one is specified", func() {
		mld.ImageRepoSecret = &v1.LocalObjectReference{
			Name: "secret",
		}

		authGetter := &auth.MockRegistryAuthGetter{}
		gomock.InOrder(
			mockAuthFactory.EXPECT().NewRegistryAuthGetterFrom(&mld).Return(authGetter),
			mockRegistry.EXPECT().ImageExists(ctx, imageName, gomock.Any(), authGetter).Return(false, nil),
		)

		exists, err := ImageExists(ctx, mockAuthFactory, mockRegistry, &mld, imageName)

		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
	})
})
