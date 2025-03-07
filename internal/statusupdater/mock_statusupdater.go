// Code generated by MockGen. DO NOT EDIT.
// Source: statusupdater.go

// Package statusupdater is a generated GoMock package.
package statusupdater

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	v1beta1 "github.com/rh-ecosystem-edge/kernel-module-management/api-hub/v1beta1"
	v1beta10 "github.com/rh-ecosystem-edge/kernel-module-management/api/v1beta1"
	v1 "k8s.io/api/apps/v1"
	v10 "k8s.io/api/core/v1"
	sets "k8s.io/apimachinery/pkg/util/sets"
	v11 "open-cluster-management.io/api/work/v1"
)

// MockModuleStatusUpdater is a mock of ModuleStatusUpdater interface.
type MockModuleStatusUpdater struct {
	ctrl     *gomock.Controller
	recorder *MockModuleStatusUpdaterMockRecorder
}

// MockModuleStatusUpdaterMockRecorder is the mock recorder for MockModuleStatusUpdater.
type MockModuleStatusUpdaterMockRecorder struct {
	mock *MockModuleStatusUpdater
}

// NewMockModuleStatusUpdater creates a new mock instance.
func NewMockModuleStatusUpdater(ctrl *gomock.Controller) *MockModuleStatusUpdater {
	mock := &MockModuleStatusUpdater{ctrl: ctrl}
	mock.recorder = &MockModuleStatusUpdaterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockModuleStatusUpdater) EXPECT() *MockModuleStatusUpdaterMockRecorder {
	return m.recorder
}

// ModuleUpdateStatus mocks base method.
func (m *MockModuleStatusUpdater) ModuleUpdateStatus(ctx context.Context, mod *v1beta10.Module, kernelMappingNodes, targetedNodes []v10.Node, dsByKernelVersion map[string]*v1.DaemonSet) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ModuleUpdateStatus", ctx, mod, kernelMappingNodes, targetedNodes, dsByKernelVersion)
	ret0, _ := ret[0].(error)
	return ret0
}

// ModuleUpdateStatus indicates an expected call of ModuleUpdateStatus.
func (mr *MockModuleStatusUpdaterMockRecorder) ModuleUpdateStatus(ctx, mod, kernelMappingNodes, targetedNodes, dsByKernelVersion interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ModuleUpdateStatus", reflect.TypeOf((*MockModuleStatusUpdater)(nil).ModuleUpdateStatus), ctx, mod, kernelMappingNodes, targetedNodes, dsByKernelVersion)
}

// MockManagedClusterModuleStatusUpdater is a mock of ManagedClusterModuleStatusUpdater interface.
type MockManagedClusterModuleStatusUpdater struct {
	ctrl     *gomock.Controller
	recorder *MockManagedClusterModuleStatusUpdaterMockRecorder
}

// MockManagedClusterModuleStatusUpdaterMockRecorder is the mock recorder for MockManagedClusterModuleStatusUpdater.
type MockManagedClusterModuleStatusUpdaterMockRecorder struct {
	mock *MockManagedClusterModuleStatusUpdater
}

// NewMockManagedClusterModuleStatusUpdater creates a new mock instance.
func NewMockManagedClusterModuleStatusUpdater(ctrl *gomock.Controller) *MockManagedClusterModuleStatusUpdater {
	mock := &MockManagedClusterModuleStatusUpdater{ctrl: ctrl}
	mock.recorder = &MockManagedClusterModuleStatusUpdaterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockManagedClusterModuleStatusUpdater) EXPECT() *MockManagedClusterModuleStatusUpdaterMockRecorder {
	return m.recorder
}

// ManagedClusterModuleUpdateStatus mocks base method.
func (m *MockManagedClusterModuleStatusUpdater) ManagedClusterModuleUpdateStatus(ctx context.Context, mcm *v1beta1.ManagedClusterModule, ownedManifestWorks []v11.ManifestWork) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ManagedClusterModuleUpdateStatus", ctx, mcm, ownedManifestWorks)
	ret0, _ := ret[0].(error)
	return ret0
}

// ManagedClusterModuleUpdateStatus indicates an expected call of ManagedClusterModuleUpdateStatus.
func (mr *MockManagedClusterModuleStatusUpdaterMockRecorder) ManagedClusterModuleUpdateStatus(ctx, mcm, ownedManifestWorks interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ManagedClusterModuleUpdateStatus", reflect.TypeOf((*MockManagedClusterModuleStatusUpdater)(nil).ManagedClusterModuleUpdateStatus), ctx, mcm, ownedManifestWorks)
}

// MockPreflightStatusUpdater is a mock of PreflightStatusUpdater interface.
type MockPreflightStatusUpdater struct {
	ctrl     *gomock.Controller
	recorder *MockPreflightStatusUpdaterMockRecorder
}

// MockPreflightStatusUpdaterMockRecorder is the mock recorder for MockPreflightStatusUpdater.
type MockPreflightStatusUpdaterMockRecorder struct {
	mock *MockPreflightStatusUpdater
}

// NewMockPreflightStatusUpdater creates a new mock instance.
func NewMockPreflightStatusUpdater(ctrl *gomock.Controller) *MockPreflightStatusUpdater {
	mock := &MockPreflightStatusUpdater{ctrl: ctrl}
	mock.recorder = &MockPreflightStatusUpdaterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPreflightStatusUpdater) EXPECT() *MockPreflightStatusUpdaterMockRecorder {
	return m.recorder
}

// PreflightPresetStatuses mocks base method.
func (m *MockPreflightStatusUpdater) PreflightPresetStatuses(ctx context.Context, pv *v1beta10.PreflightValidation, existingModules sets.String, newModules []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PreflightPresetStatuses", ctx, pv, existingModules, newModules)
	ret0, _ := ret[0].(error)
	return ret0
}

// PreflightPresetStatuses indicates an expected call of PreflightPresetStatuses.
func (mr *MockPreflightStatusUpdaterMockRecorder) PreflightPresetStatuses(ctx, pv, existingModules, newModules interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PreflightPresetStatuses", reflect.TypeOf((*MockPreflightStatusUpdater)(nil).PreflightPresetStatuses), ctx, pv, existingModules, newModules)
}

// PreflightSetVerificationStage mocks base method.
func (m *MockPreflightStatusUpdater) PreflightSetVerificationStage(ctx context.Context, preflight *v1beta10.PreflightValidation, moduleName, stage string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PreflightSetVerificationStage", ctx, preflight, moduleName, stage)
	ret0, _ := ret[0].(error)
	return ret0
}

// PreflightSetVerificationStage indicates an expected call of PreflightSetVerificationStage.
func (mr *MockPreflightStatusUpdaterMockRecorder) PreflightSetVerificationStage(ctx, preflight, moduleName, stage interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PreflightSetVerificationStage", reflect.TypeOf((*MockPreflightStatusUpdater)(nil).PreflightSetVerificationStage), ctx, preflight, moduleName, stage)
}

// PreflightSetVerificationStatus mocks base method.
func (m *MockPreflightStatusUpdater) PreflightSetVerificationStatus(ctx context.Context, preflight *v1beta10.PreflightValidation, moduleName, verificationStatus, message string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PreflightSetVerificationStatus", ctx, preflight, moduleName, verificationStatus, message)
	ret0, _ := ret[0].(error)
	return ret0
}

// PreflightSetVerificationStatus indicates an expected call of PreflightSetVerificationStatus.
func (mr *MockPreflightStatusUpdaterMockRecorder) PreflightSetVerificationStatus(ctx, preflight, moduleName, verificationStatus, message interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PreflightSetVerificationStatus", reflect.TypeOf((*MockPreflightStatusUpdater)(nil).PreflightSetVerificationStatus), ctx, preflight, moduleName, verificationStatus, message)
}

// MockPreflightOCPStatusUpdater is a mock of PreflightOCPStatusUpdater interface.
type MockPreflightOCPStatusUpdater struct {
	ctrl     *gomock.Controller
	recorder *MockPreflightOCPStatusUpdaterMockRecorder
}

// MockPreflightOCPStatusUpdaterMockRecorder is the mock recorder for MockPreflightOCPStatusUpdater.
type MockPreflightOCPStatusUpdaterMockRecorder struct {
	mock *MockPreflightOCPStatusUpdater
}

// NewMockPreflightOCPStatusUpdater creates a new mock instance.
func NewMockPreflightOCPStatusUpdater(ctrl *gomock.Controller) *MockPreflightOCPStatusUpdater {
	mock := &MockPreflightOCPStatusUpdater{ctrl: ctrl}
	mock.recorder = &MockPreflightOCPStatusUpdaterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPreflightOCPStatusUpdater) EXPECT() *MockPreflightOCPStatusUpdaterMockRecorder {
	return m.recorder
}

// PreflightOCPUpdateStatus mocks base method.
func (m *MockPreflightOCPStatusUpdater) PreflightOCPUpdateStatus(ctx context.Context, pvo *v1beta10.PreflightValidationOCP, pv *v1beta10.PreflightValidation) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PreflightOCPUpdateStatus", ctx, pvo, pv)
	ret0, _ := ret[0].(error)
	return ret0
}

// PreflightOCPUpdateStatus indicates an expected call of PreflightOCPUpdateStatus.
func (mr *MockPreflightOCPStatusUpdaterMockRecorder) PreflightOCPUpdateStatus(ctx, pvo, pv interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PreflightOCPUpdateStatus", reflect.TypeOf((*MockPreflightOCPStatusUpdater)(nil).PreflightOCPUpdateStatus), ctx, pvo, pv)
}
