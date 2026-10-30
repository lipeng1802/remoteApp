//go:build darwin && cgo

package nativejournal

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>

static CFMutableDictionaryRef journalQuery(const char *service, const char *account) {
 CFMutableDictionaryRef q=CFDictionaryCreateMutable(NULL,0,&kCFTypeDictionaryKeyCallBacks,&kCFTypeDictionaryValueCallBacks);
 CFStringRef s=CFStringCreateWithCString(NULL,service,kCFStringEncodingUTF8);
 CFDictionarySetValue(q,kSecClass,kSecClassGenericPassword);
 CFDictionarySetValue(q,kSecAttrService,s); CFRelease(s);
 // Keep command-line helpers on the classic login Keychain. Supplying an
 // LAContext selects the Data Protection Keychain on current macOS, which
 // requires restricted application/keychain-group entitlements unavailable to
 // ad-hoc PoC binaries. Explicitly fail instead of displaying authentication UI.
 CFDictionarySetValue(q,kSecUseAuthenticationUI,kSecUseAuthenticationUIFail);
 CFDictionarySetValue(q,kSecUseDataProtectionKeychain,kCFBooleanFalse);
 CFDictionarySetValue(q,kSecAttrSynchronizable,kCFBooleanFalse);
 if(account) { CFStringRef a=CFStringCreateWithCString(NULL,account,kCFStringEncodingUTF8);CFDictionarySetValue(q,kSecAttrAccount,a);CFRelease(a); }
 return q;
}
static OSStatus journalRead(const char *service,const char *account,void **data,long *size) {
 CFMutableDictionaryRef q=journalQuery(service,account);
 CFDictionarySetValue(q,kSecReturnData,kCFBooleanTrue);
 CFDictionarySetValue(q,kSecMatchLimit,kSecMatchLimitOne);
 CFTypeRef result=NULL;OSStatus s=SecItemCopyMatching(q,&result);CFRelease(q);
 if(s!=errSecSuccess)return s;
 if(!result || CFGetTypeID(result)!=CFDataGetTypeID()){if(result)CFRelease(result);return errSecDecode;}
 *size=CFDataGetLength((CFDataRef)result);
 if(*size<1 || *size>2048){CFRelease(result);return errSecDecode;}
 *data=malloc(*size);if(!*data){CFRelease(result);return errSecAllocate;}
 memcpy(*data,CFDataGetBytePtr((CFDataRef)result),*size);CFRelease(result);return errSecSuccess;
}
static OSStatus journalWrite(const char *service,const char *account,const void *bytes,long size) {
 CFMutableDictionaryRef q=journalQuery(service,account);
 CFDataRef data=CFDataCreate(NULL,bytes,size);
 const void *keys[]={kSecValueData};const void *values[]={data};
 CFDictionaryRef attrs=CFDictionaryCreate(NULL,keys,values,1,&kCFTypeDictionaryKeyCallBacks,&kCFTypeDictionaryValueCallBacks);
 OSStatus s=SecItemUpdate(q,attrs);CFRelease(attrs);
 if(s==errSecItemNotFound){
  CFDictionarySetValue(q,kSecValueData,data);
  CFDictionarySetValue(q,kSecAttrAccessible,kSecAttrAccessibleWhenUnlockedThisDeviceOnly);
  s=SecItemAdd(q,NULL);
 }
 CFRelease(data);CFRelease(q);return s;
}
static OSStatus journalEmpty(const char *service) {
 CFMutableDictionaryRef q=journalQuery(service,NULL);
 CFDictionarySetValue(q,kSecReturnAttributes,kCFBooleanTrue);
 CFTypeRef result=NULL;OSStatus s=SecItemCopyMatching(q,&result);
 if(result)CFRelease(result);CFRelease(q);return s;
}
static OSStatus journalDelete(const char *service,const char *account) {
 CFMutableDictionaryRef q=journalQuery(service,account);OSStatus s=SecItemDelete(q);CFRelease(q);return s;
}
static OSStatus journalFirstAccount(const char *service,char **account) {
 CFMutableDictionaryRef q=journalQuery(service,NULL);
 CFDictionarySetValue(q,kSecReturnAttributes,kCFBooleanTrue);
 CFDictionarySetValue(q,kSecMatchLimit,kSecMatchLimitOne);
 CFTypeRef result=NULL;OSStatus status=SecItemCopyMatching(q,&result);CFRelease(q);
 if(status!=errSecSuccess)return status;
 if(!result||CFGetTypeID(result)!=CFDictionaryGetTypeID()){if(result)CFRelease(result);return errSecDecode;}
 CFTypeRef a=CFDictionaryGetValue((CFDictionaryRef)result,kSecAttrAccount);
 CFTypeRef s=CFDictionaryGetValue((CFDictionaryRef)result,kSecAttrService);
 CFStringRef wanted=CFStringCreateWithCString(NULL,service,kCFStringEncodingUTF8);
 if(!a||!s||CFGetTypeID(a)!=CFStringGetTypeID()||!CFEqual(s,wanted)){CFRelease(wanted);CFRelease(result);return errSecDecode;}
 CFRelease(wanted);char buffer[256];
 if(!CFStringGetCString((CFStringRef)a,buffer,sizeof(buffer),kCFStringEncodingUTF8)){CFRelease(result);return errSecDecode;}
 *account=strdup(buffer);CFRelease(result);return *account?errSecSuccess:errSecAllocate;
}
*/
import "C"

import (
	"os"
	"os/user"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

type darwinValues struct {
	service   string
	dir, lock int
	test      bool
}

func openNative(scope string, test bool) (keyValues, error) {
	// Anchor at the OS per-user application-support location, no caller path.
	account, e := user.Current()
	if e != nil || account.HomeDir == "" {
		return nil, ErrUnavailable
	}
	base := filepath.Join(account.HomeDir, "Library", "Application Support")
	fd, e := unix.Open(base, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer unix.Close(fd)
	name := "RemoteAppTrustJournalPoC-v1"
	if test {
		name = "RemoteAppTrustJournalTest-v1"
	}
	if e = unix.Mkdirat(fd, name, 0700); e != nil && e != unix.EEXIST {
		return nil, ErrUnavailable
	}
	dir, e := unix.Openat(fd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, ErrUnavailable
	}
	var st unix.Stat_t
	if unix.Fstat(dir, &st) != nil || st.Uid != uint32(os.Getuid()) || st.Mode&0777 != 0700 {
		unix.Close(dir)
		return nil, ErrUnavailable
	}
	lock, e := unix.Openat(dir, scope+".lock", unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0600)
	if e != nil {
		unix.Close(dir)
		return nil, ErrUnavailable
	}
	if unix.Fstat(lock, &st) != nil || st.Uid != uint32(os.Getuid()) || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Mode&0777 != 0600 || st.Nlink != 1 || unix.Flock(lock, unix.LOCK_EX|unix.LOCK_NB) != nil {
		unix.Close(lock)
		unix.Close(dir)
		return nil, ErrUnavailable
	}
	return &darwinValues{namespace(scope, test), dir, lock, test}, nil
}

func keychainError(status C.OSStatus) error {
	if status == C.errSecInteractionNotAllowed || status == C.errSecAuthFailed {
		return ErrAuthentication
	}
	if status == C.errSecItemNotFound {
		return ErrMissing
	}
	return ErrUnavailable
}
func (d *darwinValues) checked() bool {
	var a, b unix.Stat_t
	return unix.Fstat(d.dir, &a) == nil && a.Uid == uint32(os.Getuid()) && a.Mode&0777 == 0700 &&
		unix.Fstat(d.lock, &b) == nil && b.Uid == uint32(os.Getuid()) && b.Mode&0777 == 0600 && b.Nlink == 1
}
func (d *darwinValues) Read(name string) ([]byte, error) {
	if !validName(name) || !d.checked() {
		return nil, ErrUnavailable
	}
	s, n := C.CString(d.service), C.CString(name)
	defer C.free(unsafe.Pointer(s))
	defer C.free(unsafe.Pointer(n))
	var data unsafe.Pointer
	var size C.long
	status := C.journalRead(s, n, &data, &size)
	if status != C.errSecSuccess {
		return nil, keychainError(status)
	}
	defer C.free(data)
	return C.GoBytes(data, C.int(size)), nil
}
func (d *darwinValues) Write(name string, b []byte) error {
	if !validName(name) || !d.checked() || len(b) < 1 || len(b) > chunkSize {
		return ErrUnavailable
	}
	s, n := C.CString(d.service), C.CString(name)
	defer C.free(unsafe.Pointer(s))
	defer C.free(unsafe.Pointer(n))
	status := C.journalWrite(s, n, unsafe.Pointer(&b[0]), C.long(len(b)))
	if status != C.errSecSuccess {
		return keychainError(status)
	}
	return nil
}
func (d *darwinValues) Remove(name string) error {
	if !validName(name) || !d.checked() {
		return ErrUnavailable
	}
	s, n := C.CString(d.service), C.CString(name)
	defer C.free(unsafe.Pointer(s))
	defer C.free(unsafe.Pointer(n))
	status := C.journalDelete(s, n)
	if status != C.errSecSuccess {
		return keychainError(status)
	}
	return nil
}
func (d *darwinValues) Empty() (bool, error) {
	if !d.checked() {
		return false, ErrUnavailable
	}
	s := C.CString(d.service)
	defer C.free(unsafe.Pointer(s))
	status := C.journalEmpty(s)
	if status == C.errSecItemNotFound {
		return true, nil
	}
	if status == C.errSecSuccess {
		return false, nil
	}
	return false, keychainError(status)
}
func (d *darwinValues) ClearTest() error {
	if !d.test || !d.checked() {
		return ErrUnavailable
	}
	s := C.CString(d.service)
	defer C.free(unsafe.Pointer(s))
	// Delete exact validated accounts and confirm exhaustion. Do not accept a
	// broad delete acknowledgement as proof that all native chunks are gone.
	seen := map[string]bool{}
	for count := 0; count < 128; count++ {
		var account *C.char
		status := C.journalFirstAccount(s, &account)
		if status == C.errSecItemNotFound {
			return nil
		}
		if status != C.errSecSuccess {
			return keychainError(status)
		}
		name := C.GoString(account)
		C.free(unsafe.Pointer(account))
		if !validName(name) || seen[name] {
			return ErrUnavailable
		}
		seen[name] = true
		if d.Remove(name) != nil {
			return ErrUnavailable
		}
	}
	return ErrUnavailable
}
func (d *darwinValues) Close() error {
	e := unix.Close(d.lock)
	if next := unix.Close(d.dir); e == nil {
		e = next
	}
	return e
}
