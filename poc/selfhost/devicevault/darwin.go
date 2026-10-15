//go:build darwin && cgo

package devicevault

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Security -framework CoreFoundation -framework Foundation -framework LocalAuthentication
#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#include <Security/Security.h>
#include <stdlib.h>
#include <string.h>

static CFMutableDictionaryRef vaultQuery(const char *account) {
 CFMutableDictionaryRef q=CFDictionaryCreateMutable(NULL,0,&kCFTypeDictionaryKeyCallBacks,&kCFTypeDictionaryValueCallBacks);
 CFStringRef service=CFStringCreateWithCString(NULL,"RemoteApp/ConnectionPoC/v1",kCFStringEncodingUTF8);
 CFStringRef name=CFStringCreateWithCString(NULL,account,kCFStringEncodingUTF8);
 CFDictionarySetValue(q,kSecClass,kSecClassGenericPassword);
 CFDictionarySetValue(q,kSecAttrService,service);CFRelease(service);
 CFDictionarySetValue(q,kSecAttrAccount,name);CFRelease(name);
 CFDictionarySetValue(q,kSecAttrSynchronizable,kCFBooleanFalse);
 LAContext *auth=[[LAContext alloc] init];auth.interactionNotAllowed=YES;
 CFDictionarySetValue(q,kSecUseAuthenticationContext,(CFTypeRef)auth);[auth release];
 return q;
}
static OSStatus vaultRead(const char *account,void **data,long *size) {
 CFMutableDictionaryRef q=vaultQuery(account);
 CFDictionarySetValue(q,kSecReturnData,kCFBooleanTrue);
 CFDictionarySetValue(q,kSecMatchLimit,kSecMatchLimitOne);
 CFTypeRef result=NULL;OSStatus status=SecItemCopyMatching(q,&result);CFRelease(q);
 if(status!=errSecSuccess)return status;
 if(!result||CFGetTypeID(result)!=CFDataGetTypeID()){if(result)CFRelease(result);return errSecDecode;}
 *size=CFDataGetLength((CFDataRef)result);
 if(*size!=32){CFRelease(result);return errSecDecode;}
 *data=malloc(32);if(!*data){CFRelease(result);return errSecAllocate;}
 memcpy(*data,CFDataGetBytePtr((CFDataRef)result),32);CFRelease(result);return errSecSuccess;
}
static OSStatus vaultAdd(const char *account,const void *bytes) {
 CFMutableDictionaryRef q=vaultQuery(account);
 CFDataRef data=CFDataCreate(NULL,bytes,32);
 CFDictionarySetValue(q,kSecValueData,data);
 CFDictionarySetValue(q,kSecAttrAccessible,kSecAttrAccessibleWhenUnlockedThisDeviceOnly);
 OSStatus status=SecItemAdd(q,NULL);CFRelease(data);CFRelease(q);return status;
}
static OSStatus vaultDelete(const char *account) {
 CFMutableDictionaryRef q=vaultQuery(account);OSStatus status=SecItemDelete(q);CFRelease(q);return status;
}
*/
import "C"

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"unsafe"
)

var ErrUnavailable = errors.New("device_vault_unavailable")
var ErrLogonUnavailable = errors.New("interactive_logon_required")

func validScope(scope string) bool {
	raw, err := hex.DecodeString(scope)
	return err == nil && len(raw) == 16 && hex.EncodeToString(raw) == scope
}

func vaultError(status C.OSStatus) error {
	if status == C.errSecInteractionNotAllowed || status == C.errSecAuthFailed || status == C.errSecNotAvailable {
		return ErrLogonUnavailable
	}
	return ErrUnavailable
}

func readSeed(scope string) ([]byte, C.OSStatus) {
	account := C.CString(scope)
	defer C.free(unsafe.Pointer(account))
	var data unsafe.Pointer
	var size C.long
	status := C.vaultRead(account, &data, &size)
	if status != C.errSecSuccess {
		return nil, status
	}
	defer C.free(data)
	return C.GoBytes(data, 32), status
}

func Load(scope string, create bool) ([]byte, error) {
	if !validScope(scope) {
		return nil, ErrUnavailable
	}
	seed, status := readSeed(scope)
	if status == C.errSecSuccess {
		if create {
			for i := range seed {
				seed[i] = 0
			}
			return nil, ErrUnavailable
		}
		return seed, nil
	}
	if status != C.errSecItemNotFound || !create {
		return nil, vaultError(status)
	}
	seed = make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return nil, ErrUnavailable
	}
	account := C.CString(scope)
	status = C.vaultAdd(account, unsafe.Pointer(&seed[0]))
	C.free(unsafe.Pointer(account))
	if status != C.errSecSuccess {
		for i := range seed {
			seed[i] = 0
		}
		return nil, vaultError(status)
	}
	stored, status := readSeed(scope)
	if status != C.errSecSuccess || len(stored) != len(seed) {
		for i := range seed {
			seed[i] = 0
		}
		return nil, vaultError(status)
	}
	for i := range stored {
		if stored[i] != seed[i] {
			for j := range seed {
				seed[j] = 0
			}
			return nil, ErrUnavailable
		}
	}
	return seed, nil
}

func Delete(scope string) error {
	if !validScope(scope) {
		return ErrUnavailable
	}
	account := C.CString(scope)
	status := C.vaultDelete(account)
	C.free(unsafe.Pointer(account))
	if status != C.errSecSuccess {
		return vaultError(status)
	}
	return nil
}
