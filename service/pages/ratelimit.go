package pages

import (
	"sync"
	"time"
)

const (
	loginMaxFailures = 5
	loginLockWindow  = 15 * time.Minute
)

type loginFail struct {
	count int
	first time.Time
	last  time.Time
}

var (
	loginMu    sync.Mutex
	loginFails = map[string]*loginFail{}
)

// loginKey 限流键：IP + 用户名。只按 IP 会让同一 NAT 后的用户互相影响。
func loginKey(ip, username string) string {
	return ip + "|" + username
}

// checkLoginAllowed 判断是否允许尝试，返回 (允许, 剩余锁定秒数)
func checkLoginAllowed(ip, username string) (bool, int) {
	loginMu.Lock()
	defer loginMu.Unlock()

	k := loginKey(ip, username)
	f, ok := loginFails[k]
	if !ok {
		return true, 0
	}
	if time.Since(f.first) > loginLockWindow {
		delete(loginFails, k)
		return true, 0
	}
	if f.count >= loginMaxFailures {
		remain := loginLockWindow - time.Since(f.first)
		if remain < 0 {
			remain = 0
		}
		return false, int(remain.Seconds())
	}
	return true, 0
}

func recordLoginFailure(ip, username string) {
	loginMu.Lock()
	defer loginMu.Unlock()

	k := loginKey(ip, username)
	now := time.Now()
	f, ok := loginFails[k]
	if !ok || now.Sub(f.first) > loginLockWindow {
		loginFails[k] = &loginFail{count: 1, first: now, last: now}
		return
	}
	f.count++
	f.last = now
}

func resetLoginFailures(ip, username string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginFails, loginKey(ip, username))
}

// 定期清理过期记录，避免内存泄漏
func init() {
	go func() {
		t := time.NewTicker(10 * time.Minute)
		defer t.Stop()
		for range t.C {
			loginMu.Lock()
			for k, f := range loginFails {
				if time.Since(f.first) > loginLockWindow*2 {
					delete(loginFails, k)
				}
			}
			loginMu.Unlock()
		}
	}()
}
