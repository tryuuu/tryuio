package replication

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/tryuuu/tryuio/internal/domain"
	"github.com/tryuuu/tryuio/internal/repository"
)

// Replicator は PUT/DELETE を他ノードへ複製し、復帰ノードへの再同期を担う。
type Replicator struct {
	pm     *PeerManager
	repo   repository.ObjectRepository
	apiKey string
	client *http.Client
}

func NewReplicator(pm *PeerManager, repo repository.ObjectRepository, apiKey string) *Replicator {
	r := &Replicator{
		pm:     pm,
		repo:   repo,
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
	pm.OnRecovery(r.recover)
	return r
}

// ReplicatePut はオンラインの全ピアに PUT を非同期で転送する。
func (r *Replicator) ReplicatePut(obj *domain.Object) {
	for _, peer := range r.pm.OnlinePeers() {
		go func(peerURL string) {
			if err := r.sendPut(peerURL, obj); err != nil {
				log.Printf("[replication] PUT %s/%s to %s failed: %v", obj.Bucket, obj.Key, peerURL, err)
			}
		}(peer)
	}
}

// ReplicateDelete はオンラインの全ピアに DELETE を非同期で転送する。
func (r *Replicator) ReplicateDelete(bucket, key string) {
	for _, peer := range r.pm.OnlinePeers() {
		go func(peerURL string) {
			if err := r.sendDelete(peerURL, bucket, key); err != nil {
				log.Printf("[replication] DELETE %s/%s to %s failed: %v", bucket, key, peerURL, err)
			}
		}(peer)
	}
}

func (r *Replicator) sendPut(peerURL string, obj *domain.Object) error {
	url := fmt.Sprintf("%s/%s/%s", peerURL, obj.Bucket, obj.Key)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(obj.Body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", obj.ContentType)
	req.Header.Set("X-Replicated", "true")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("peer returned %d", resp.StatusCode)
	}
	return nil
}

func (r *Replicator) sendDelete(peerURL, bucket, key string) error {
	url := fmt.Sprintf("%s/%s/%s", peerURL, bucket, key)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("X-Replicated", "true")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	// 404 は既に削除済みなので正常とみなす
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("peer returned %d", resp.StatusCode)
	}
	return nil
}

// recover は復帰したピアとの差分を解消する。
// - 自ノードにあってピアにないファイル → ピアへ PUT
// - ピアにあって自ノードにないファイル → ピアから DELETE（オフライン中に削除されたもの）
func (r *Replicator) recover(peerURL string) {
	selfList, err := r.repo.List()
	if err != nil {
		log.Printf("[recovery] list self failed: %v", err)
		return
	}
	peerList, err := r.fetchList(peerURL)
	if err != nil {
		log.Printf("[recovery] list peer %s failed: %v", peerURL, err)
		return
	}

	selfSet := make(map[string]bool, len(selfList))
	for _, f := range selfList {
		selfSet[f] = true
	}

	peerSet := make(map[string]bool, len(peerList))
	for _, f := range peerList {
		peerSet[f] = true
	}

	// 自ノードにあってピアにない → PUT で補完
	for _, path := range selfList {
		if peerSet[path] {
			continue
		}
		bucket, key, ok := splitBucketKey(path)
		if !ok {
			continue
		}
		obj, err := r.repo.Get(bucket, key)
		if err != nil {
			log.Printf("[recovery] get %s failed: %v", path, err)
			continue
		}
		if err := r.sendPut(peerURL, obj); err != nil {
			log.Printf("[recovery] sync %s to %s failed: %v", path, peerURL, err)
		} else {
			log.Printf("[recovery] synced %s to %s", path, peerURL)
		}
	}

	// ピアにあって自ノードにない → ピアから DELETE（オフライン中に削除済み）
	for _, path := range peerList {
		if selfSet[path] {
			continue
		}
		bucket, key, ok := splitBucketKey(path)
		if !ok {
			continue
		}
		if err := r.sendDelete(peerURL, bucket, key); err != nil {
			log.Printf("[recovery] delete %s from %s failed: %v", path, peerURL, err)
		} else {
			log.Printf("[recovery] deleted stale %s from %s", path, peerURL)
		}
	}
}

func (r *Replicator) fetchList(peerURL string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, peerURL+"/list", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var list []string
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func splitBucketKey(path string) (bucket, key string, ok bool) {
	idx := strings.Index(path, "/")
	if idx < 0 {
		return "", "", false
	}
	return path[:idx], path[idx+1:], true
}
