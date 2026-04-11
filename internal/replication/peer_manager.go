package replication

import (
	"log"
	"net/http"
	"sync"
	"time"
)


// PeerManager は各ピアのオンライン/オフライン状態を管理する。
type PeerManager struct {
	peers      []string
	status     map[string]bool
	mu         sync.RWMutex
	onRecovery func(peerURL string)
}

func NewPeerManager(peers []string) *PeerManager {
	status := make(map[string]bool)
	for _, p := range peers {
		status[p] = false // 起動時はオフライン扱いにして初回ヘルスチェックで確定させる
	}
	return &PeerManager{peers: peers, status: status}
}

// OnRecovery はピアが復帰したときに呼ばれるコールバックを登録する。
func (pm *PeerManager) OnRecovery(fn func(peerURL string)) {
	pm.onRecovery = fn
}

// Start はバックグラウンドで定期ヘルスチェックを開始する。
func (pm *PeerManager) Start(interval time.Duration) {
	go func() {
		for {
			pm.checkAll()
			time.Sleep(interval)
		}
	}()
}

func (pm *PeerManager) checkAll() {
	var wg sync.WaitGroup
	for _, peer := range pm.peers {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			online := pm.checkPeer(p)
			pm.mu.Lock()
			wasOnline := pm.status[p]
			pm.status[p] = online
			pm.mu.Unlock()
			if online && !wasOnline {
				log.Printf("[peer] recovered: %s", p)
				if pm.onRecovery != nil {
					go pm.onRecovery(p)
				}
			}
			if !online && wasOnline {
				log.Printf("[peer] down: %s", p)
			}
		}(peer)
	}
	wg.Wait()
}

func (pm *PeerManager) checkPeer(peerURL string) bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(peerURL + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// OnlinePeers はオンラインのピア URL 一覧を返す。
func (pm *PeerManager) OnlinePeers() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var online []string
	for _, p := range pm.peers {
		if pm.status[p] {
			online = append(online, p)
		}
	}
	return online
}
