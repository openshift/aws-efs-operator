// Code originally generated by MockGen.
// Source: sigs.k8s.io/controller-runtime/pkg/client (interfaces: Client)

// Subsequently augmented by hand.
// Please add enhancements after END OF GENERATED CODE

// Package fixtures is a generated GoMock package.
package fixtures

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	runtime "k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	reflect "reflect"
	client "sigs.k8s.io/controller-runtime/pkg/client"

	// Added by hand
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockClient is a mock of Client interface
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// Create mocks base method
func (m *MockClient) Create(arg0 context.Context, arg1 runtime.Object, arg2 ...client.CreateOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Create", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create
func (mr *MockClientMockRecorder) Create(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockClient)(nil).Create), varargs...)
}

// Delete mocks base method
func (m *MockClient) Delete(arg0 context.Context, arg1 runtime.Object, arg2 ...client.DeleteOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Delete", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockClientMockRecorder) Delete(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockClient)(nil).Delete), varargs...)
}

// DeleteAllOf mocks base method
func (m *MockClient) DeleteAllOf(arg0 context.Context, arg1 runtime.Object, arg2 ...client.DeleteAllOfOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DeleteAllOf", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteAllOf indicates an expected call of DeleteAllOf
func (mr *MockClientMockRecorder) DeleteAllOf(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAllOf", reflect.TypeOf((*MockClient)(nil).DeleteAllOf), varargs...)
}

// Get mocks base method
func (m *MockClient) Get(arg0 context.Context, arg1 types.NamespacedName, arg2 runtime.Object) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Get indicates an expected call of Get
func (mr *MockClientMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockClient)(nil).Get), arg0, arg1, arg2)
}

// List mocks base method
func (m *MockClient) List(arg0 context.Context, arg1 runtime.Object, arg2 ...client.ListOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "List", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// List indicates an expected call of List
func (mr *MockClientMockRecorder) List(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockClient)(nil).List), varargs...)
}

// Patch mocks base method
func (m *MockClient) Patch(arg0 context.Context, arg1 runtime.Object, arg2 client.Patch, arg3 ...client.PatchOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Patch", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Patch indicates an expected call of Patch
func (mr *MockClientMockRecorder) Patch(arg0, arg1, arg2 interface{}, arg3 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1, arg2}, arg3...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Patch", reflect.TypeOf((*MockClient)(nil).Patch), varargs...)
}

// Status mocks base method
func (m *MockClient) Status() client.StatusWriter {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Status")
	ret0, _ := ret[0].(client.StatusWriter)
	return ret0
}

// Status indicates an expected call of Status
func (mr *MockClientMockRecorder) Status() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Status", reflect.TypeOf((*MockClient)(nil).Status))
}

// Update mocks base method
func (m *MockClient) Update(arg0 context.Context, arg1 runtime.Object, arg2 ...client.UpdateOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Update", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update
func (mr *MockClientMockRecorder) Update(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockClient)(nil).Update), varargs...)
}

////////////////////////
// END OF GENERATED CODE
////////////////////////

// clientError will answer APIStatus questions like IsNotFound according to its `status`.
type clientError struct {
	reason metav1.StatusReason
}

// Status implements APIStatus
func (ce clientError) Status() metav1.Status {
	return metav1.Status{Reason: ce.reason}
}

// Error implements error
func (ce clientError) Error() string {
	return string(ce.reason)
}

// A couple of error stubs that can be returned from the Client.

// NotFound stub API response
var NotFound error = clientError{reason: metav1.StatusReasonNotFound}
// AlreadyExists stub API response
var AlreadyExists error = clientError{reason: metav1.StatusReasonAlreadyExists}