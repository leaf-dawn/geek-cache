package geek

import (
	"fmt"
	"log"
	"sync"
)

var (
	lock   sync.RWMutex
	groups = make(map[string]*Group)
)

type Group struct {
	name      string     // group name
	getter    Getter     // 缓存未名中时的callback
	mainCache cache      // main cache
	peers     PeerPicker // pick function
}

func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called multiple times")
	}
	g.peers = peers
}

// NewGroup 新创建一个Group
// 如果存在同名的group会进行覆盖
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	lock.Lock()
	defer lock.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
	}
	groups[name] = g
	return g
}

func GetGroup(name string) *Group {
	lock.RLock()
	g := groups[name]
	lock.RUnlock()
	return g
}

func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeekCache] hit")
		return v, nil
	}
	return g.load(key)
}

// get from peer first, then get locally
func (g *Group) load(key string) (ByteView, error) {
	if g.peers != nil {
		if peer, ok := g.peers.PickPeer(key); ok {
			if value, err := g.getFromPeer(peer, key); err == nil {
				return value, nil
			} else {
				log.Println("[GeekCache] Failed to get from peer", err)
			}
		}
	}
	return g.getLocally(key)
}

func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{
		b: bytes,
	}, nil
}

func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	bw := ByteView{cloneBytes(bytes)}
	g.populateCache(key, bw)
	return bw, nil
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

// Getter loads data for a key
// call back when a key cache missed
// impl by user
type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}
