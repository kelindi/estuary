package node

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	peering "github.com/application-research/estuary/node/modules/peering"

	"github.com/application-research/estuary/config"

	rcmgr "github.com/application-research/estuary/node/modules/lp2p"
	migratebs "github.com/application-research/estuary/util/migratebs"
	"github.com/application-research/filclient/keystore"
	autobatch "github.com/application-research/go-bs-autobatch"
	lmdb "github.com/filecoin-project/go-bs-lmdb"
	badgerbs "github.com/filecoin-project/lotus/blockstore/badger"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/wallet"
	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	nsds "github.com/ipfs/go-datastore/namespace"
	flatfs "github.com/ipfs/go-ds-flatfs"
	levelds "github.com/ipfs/go-ds-leveldb"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-provider/batched"
	"github.com/ipfs/go-ipfs-provider/queue"
	logging "github.com/ipfs/go-log/v2"
	metri "github.com/ipfs/go-metrics-interface"
	mprome "github.com/ipfs/go-metrics-prometheus"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	metrics "github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/multiformats/go-multiaddr"
	bsm "github.com/whyrusleeping/go-bs-measure"
	"golang.org/x/xerrors"
)

var log = logging.Logger("est-node")

var bootstrappers = []string{
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
}

var peeringPeers = []peering.PeeringPeer{
	//Cloudflare
	{ID: "QmcfgsJsMtx6qJb74akCw1M24X1zFwgGo11h1cuhwQjtJP", Addrs: []string{"/ip6/2606:4700:60::6/tcp/4009", "/ip4/172.65.0.13/tcp/4009"}},

	//	NFT storage
	{ID: "12D3KooWEGeZ19Q79NdzS6CJBoCwFZwujqi5hoK8BtRcLa48fJdu", Addrs: []string{"/ip4/145.40.96.233/tcp/4001"}},
	{ID: "12D3KooWBnmsaeNRP6SCdNbhzaNHihQQBPDhmDvjVGsR1EbswncV", Addrs: []string{"/ip4/147.75.87.85/tcp/4001"}},
	{ID: "12D3KooWDLYiAdzUdM7iJHhWu5KjmCN62aWd7brQEQGRWbv8QcVb", Addrs: []string{"/ip4/136.144.57.203/tcp/4001"}},
	{ID: "12D3KooWFZmGztVoo2K1BcAoDEUmnp7zWFhaK5LcRHJ8R735T3eY", Addrs: []string{"/ip4/145.40.69.29/tcp/4001"}},
	{ID: "12D3KooWRJpsEsBtJ1TNik2zgdirqD4KFq5V4ar2vKCrEXUqFXPP", Addrs: []string{"/ip4/139.178.70.235/tcp/4001"}},
	{ID: "12D3KooWNxUGEN1SzRuwkJdbMDnHEVViXkRQEFCSuHRTdjFvD5uw", Addrs: []string{"/ip4/145.40.67.89/tcp/4001"}},
	{ID: "12D3KooWMZmMp9QwmfJdq3aXXstMbTCCB3FTWv9SNLdQGqyPMdUw", Addrs: []string{"/ip4/145.40.69.133/tcp/4001"}},
	{ID: "12D3KooWCpu8Nk4wmoXSsVeVSVzVHmrwBnEoC9jpcVpeWP7n67Bt", Addrs: []string{"/ip4/145.40.69.171/tcp/4001"}},
	{ID: "12D3KooWGx5pFFG7W2EG8N6FFwRLh34nHcCLMzoBSMSSpHcJYN7G", Addrs: []string{"/ip4/145.40.90.235/tcp/4001"}},
	{ID: "12D3KooWQsVxhA43ZjGNUDfF9EEiNYxb1PVEgCBMNj87E9cg92vT", Addrs: []string{"/ip4/139.178.69.135/tcp/4001"}},
	{ID: "12D3KooWMSrRXHgbBTsNGfxG1E44fLB6nJ5wpjavXj4VGwXKuz9X", Addrs: []string{"/ip4/147.75.32.99/tcp/4001"}},
	{ID: "12D3KooWE48wcXK7brQY1Hw7LhjF3xdiFegLnCAibqDtyrgdxNgn", Addrs: []string{"/ip4/147.75.86.227/tcp/4001"}},
	{ID: "12D3KooWSGCJYbM6uCvCF7cGWSitXSJTgEb7zjVCaxDyYNASTa8i", Addrs: []string{"/ip4/136.144.55.33/tcp/4001"}},
	{ID: "12D3KooWJbARcvvEEF4AAqvAEaVYRkEUNPC3Rv3joebqfPh4LaKq", Addrs: []string{"/ip4/136.144.57.127/tcp/4001"}},
	{ID: "12D3KooWNcshtC1XTbPxew2kq3utG2rRGLeMN8y5vSfAMTJMV7fE", Addrs: []string{"/ip4/147.75.87.249/tcp/4001"}},

	// 	Pinata
	{ID: "QmWaik1eJcGHq1ybTWe7sezRfqKNcDRNkeBaLnGwQJz1Cj", Addrs: []string{"/dnsaddr/fra1-1.hostnodes.pinata.cloud"}},
	{ID: "QmNfpLrQQZr5Ns9FAJKpyzgnDL2GgC6xBug1yUZozKFgu4", Addrs: []string{"/dnsaddr/fra1-2.hostnodes.pinata.cloud"}},
	{ID: "QmPo1ygpngghu5it8u4Mr3ym6SEU2Wp2wA66Z91Y1S1g29", Addrs: []string{"/dnsaddr/fra1-3.hostnodes.pinata.cloud"}},
	{ID: "QmRjLSisUCHVpFa5ELVvX3qVPfdxajxWJEHs9kN3EcxAW6", Addrs: []string{"/dnsaddr/nyc1-1.hostnodes.pinata.cloud"}},
	{ID: "QmPySsdmbczdZYBpbi2oq2WMJ8ErbfxtkG8Mo192UHkfGP", Addrs: []string{"/dnsaddr/nyc1-2.hostnodes.pinata.cloud"}},
	{ID: "QmSarArpxemsPESa6FNkmuu9iSE1QWqPX2R3Aw6f5jq4D5", Addrs: []string{"/dnsaddr/nyc1-3.hostnodes.pinata.cloud"}},

	//	Protocol Labs
	{ID: "QmUEMvxS2e7iDrereVYc5SWPauXPyNwxcy9BXZrC1QTcHE", Addrs: []string{"/dns/cluster0.fsn.dwebops.pub"}},
	{ID: "QmNSYxZAiJHeLdkBg38roksAR9So7Y5eojks1yjEcUtZ7i", Addrs: []string{"/dns/cluster1.fsn.dwebops.pub"}},
	{ID: "QmUd6zHcbkbcs7SMxwLs48qZVX3vpcM8errYS7xEczwRMA", Addrs: []string{"/dns/cluster2.fsn.dwebops.pub"}},
	{ID: "QmbVWZQhCGrS7DhgLqWbgvdmKN7JueKCREVanfnVpgyq8x", Addrs: []string{"/dns/cluster3.fsn.dwebops.pub"}},
	{ID: "QmdnXwLrC8p1ueiq2Qya8joNvk3TVVDAut7PrikmZwubtR", Addrs: []string{"/dns/cluster4.fsn.dwebops.pub"}},
	{ID: "12D3KooWCRscMgHgEo3ojm8ovzheydpvTEqsDtq7Vby38cMHrYjt", Addrs: []string{"/dns4/nft-storage-am6.nft.dwebops.net/tcp/18402"}},
	{ID: "12D3KooWQtpvNvUYFzAo1cRYkydgk15JrMSHp6B6oujqgYSnvsVm", Addrs: []string{"/dns4/nft-storage-dc13.nft.dwebops.net/tcp/18402"}},
	{ID: "12D3KooWQcgCwNCTYkyLXXQSZuL5ry1TzpM8PRe9dKddfsk1BxXZ", Addrs: []string{"/dns4/nft-storage-sv15.nft.dwebops.net/tcp/18402"}},

	//	Textile
	{ID: "QmR69wtWUMm1TWnmuD4JqC1TWLZcc8iR2KrTenfZZbiztd", Addrs: []string{"/ip4/104.210.43.77"}},

	//	Web3.Storage
	{ID: "12D3KooWR19qPPiZH4khepNjS3CLXiB7AbrbAD4ZcDjN1UjGUNE1", Addrs: []string{"/ip4/139.178.69.155/tcp/4001"}},
	{ID: "12D3KooWEDMw7oRqQkdCJbyeqS5mUmWGwTp8JJ2tjCzTkHboF6wK", Addrs: []string{"/ip4/139.178.68.91/tcp/4001"}},
	{ID: "12D3KooWPySxxWQjBgX9Jp6uAHQfVmdq8HG1gVvS1fRawHNSrmqW", Addrs: []string{"/ip4/147.75.33.191/tcp/4001"}},
	{ID: "12D3KooWNuoVEfVLJvU3jWY2zLYjGUaathsecwT19jhByjnbQvkj", Addrs: []string{"/ip4/147.75.32.73/tcp/4001"}},
	{ID: "12D3KooWSnniGsyAF663gvHdqhyfJMCjWJv54cGSzcPiEMAfanvU", Addrs: []string{"/ip4/145.40.89.195/tcp/4001"}},
	{ID: "12D3KooWKytRAd2ujxhGzaLHKJuje8sVrHXvjGNvHXovpar5KaKQ", Addrs: []string{"/ip4/136.144.56.153/tcp/4001"}},

	//	Estuary
	{ID: "12D3KooWCVXs8P7iq6ao4XhfAmKWrEeuKFWCJgqe9jGDMTqHYBjw", Addrs: []string{"/ip4/139.178.68.217/tcp/6744"}},
	{ID: "12D3KooWGBWx9gyUFTVQcKMTenQMSyE2ad9m7c9fpjS4NMjoDien", Addrs: []string{"/ip4/147.75.49.71/tcp/6745"}},
	{ID: "12D3KooWFrnuj5o3tx4fGD2ZVJRyDqTdzGnU3XYXmBbWbc8Hs8Nd", Addrs: []string{"/ip4/147.75.86.255/tcp/6745"}},
	{ID: "12D3KooWN8vAoGd6eurUSidcpLYguQiGZwt4eVgDvbgaS7kiGTup", Addrs: []string{"/ip4/3.134.223.177/tcp/6745"}},
	{ID: "12D3KooWLV128pddyvoG6NBvoZw7sSrgpMTPtjnpu3mSmENqhtL7", Addrs: []string{"/ip4/35.74.45.12/udp/6746/quic"}},
}

var BootstrapPeers []peer.AddrInfo

func init() {
	if err := mprome.Inject(); err != nil {
		panic(err)
	}

	for _, bsp := range bootstrappers {
		ma, err := multiaddr.NewMultiaddr(bsp)
		if err != nil {
			log.Errorf("failed to parse bootstrap address: ", err)
			continue
		}

		ai, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			log.Errorf("failed to create address info: ", err)
			continue
		}

		BootstrapPeers = append(BootstrapPeers, *ai)
	}
}

type EstuaryBlockstore interface {
	blockstore.Blockstore
	DeleteMany(context.Context, []cid.Cid) error
}

type NodeInitializer interface {
	BlockstoreWrap(blockstore.Blockstore) (blockstore.Blockstore, error)
	KeyProviderFunc(context.Context) (<-chan cid.Cid, error)
	Config() *config.Node
}

type Node struct {
	Dht      *dht.IpfsDHT
	Provider *batched.BatchProvidingSystem
	FullRT   *fullrt.FullRT
	FilDht   *dht.IpfsDHT
	Host     host.Host

	// Set for gathering disk usage

	StorageDir string
	//Lmdb      *lmdb.Blockstore
	Datastore datastore.Batching

	Blockstore      blockstore.Blockstore
	Bitswap         *bitswap.Bitswap
	NotifBlockstore *NotifyBlockstore

	Wallet *wallet.LocalWallet

	Bwc     *metrics.BandwidthCounter
	Peering *peering.EstuaryPeeringService
	Config  *config.Node
}

func Setup(ctx context.Context, init NodeInitializer) (*Node, error) {
	cfg := init.Config()

	peerkey, err := loadOrInitPeerKey(cfg.Libp2pKeyFile)
	if err != nil {
		return nil, err
	}

	ds, err := levelds.NewDatastore(cfg.DatastoreDir, nil)
	if err != nil {
		return nil, err
	}

	var rcm network.ResourceManager
	if cfg.NoLimiter {
		rcm = network.NullResourceManager
		log.Warnf("starting node with no resource limits")
	} else {
		rcm, err = rcmgr.NewResourceManager(cfg.GetLimiter())
		if err != nil {
			return nil, err
		}
	}

	bwc := metrics.NewBandwidthCounter()

	cmgr, err := connmgr.NewConnManager(cfg.ConnectionManager.LowWater, cfg.ConnectionManager.HighWater)
	if err != nil {
		return nil, err
	}
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(cfg.ListenAddrs...),
		libp2p.NATPortMap(),
		libp2p.ConnectionManager(cmgr),
		libp2p.Identity(peerkey),
		libp2p.BandwidthReporter(bwc),
		libp2p.DefaultTransports,
		libp2p.ResourceManager(rcm),
	}

	if len(cfg.AnnounceAddrs) > 0 {
		var addrs []multiaddr.Multiaddr
		for _, anna := range cfg.AnnounceAddrs {
			a, err := multiaddr.NewMultiaddr(anna)
			if err != nil {
				return nil, fmt.Errorf("failed to parse announce addr: %w", err)
			}
			addrs = append(addrs, a)
		}
		opts = append(opts, libp2p.AddrsFactory(func([]multiaddr.Multiaddr) []multiaddr.Multiaddr {
			return addrs
		}))
	}

	h, err := libp2p.New(opts...)

	//	peering service
	peerServ := peering.NewEstuaryPeeringService(h)

	peeringPeerList := append(cfg.PeeringPeers, peeringPeers...)

	//	add the peers
	for _, addrInfo := range peeringPeerList {
		addrs, err := toMultiAddresses(addrInfo.Addrs)
		if err != nil {
			return nil, fmt.Errorf("failed to parse peering peers multi addr: %w", err)
		}
		addrInfoId, err := peer.Decode(addrInfo.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to parse peering peers multi addr ID: %w", err)
		}
		peerServ.AddPeer(peer.AddrInfo{ID: addrInfoId, Addrs: addrs})
	}

	errOnPeerStar := peerServ.Start()
	if errOnPeerStar != nil {
		log.Warn(errOnPeerStar)
	}

	if err != nil {
		return nil, err
	}

	dhtopts := fullrt.DHTOption(
		//dht.Validator(in.Validator),
		dht.Datastore(ds),
		dht.BootstrapPeers(BootstrapPeers...),
		dht.BucketSize(20),
	)

	frt, err := fullrt.NewFullRT(h, dht.DefaultPrefix, dhtopts)
	if err != nil {
		return nil, xerrors.Errorf("constructing fullrt: %w", err)
	}

	ipfsdht, err := dht.New(ctx, h, dht.Datastore(ds))
	if err != nil {
		return nil, xerrors.Errorf("constructing dht: %w", err)
	}

	filopts := []dht.Option{dht.Mode(dht.ModeAuto),
		dht.Datastore(nsds.Wrap(ds, datastore.NewKey("fildht"))),
		dht.Validator(record.NamespacedValidator{
			"pk": record.PublicKeyValidator{},
		}),
		dht.ProtocolPrefix("/fil/kad/testnetnet"),
		dht.QueryFilter(dht.PublicQueryFilter),
		dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
		dht.DisableProviders(),
		dht.DisableValues()}
	fildht, err := dht.New(ctx, h, filopts...)
	if err != nil {
		return nil, err
	}

	mbs, stordir, err := loadBlockstore(cfg.Blockstore, cfg.WriteLogDir, cfg.HardFlushWriteLog, cfg.WriteLogTruncate, cfg.NoBlockstoreCache)
	if err != nil {
		return nil, err
	}

	var blkst blockstore.Blockstore = mbs
	wrapper, err := init.BlockstoreWrap(blkst)
	if err != nil {
		return nil, err
	}
	blkst = wrapper

	bsnet := bsnet.NewFromIpfsHost(h, frt)

	peerwork := cfg.Bitswap.MaxOutstandingBytesPerPeer
	if peerwork == 0 {
		peerwork = 5 << 20
	}

	bsopts := []bitswap.Option{
		bitswap.EngineBlockstoreWorkerCount(600),
		bitswap.TaskWorkerCount(600),
		bitswap.MaxOutstandingBytesPerPeer(int(peerwork)),
	}

	if tms := cfg.Bitswap.TargetMessageSize; tms != 0 {
		bsopts = append(bsopts, bitswap.WithTargetMessageSize(tms))
	}

	bsctx := metri.CtxScope(ctx, "estuary.exch")
	bswap := bitswap.New(bsctx, bsnet, blkst, bsopts...)

	wallet, err := setupWallet(cfg.WalletDir)
	if err != nil {
		return nil, err
	}

	provq, err := queue.NewQueue(context.Background(), "provq", ds)
	if err != nil {
		return nil, err
	}

	prov, err := batched.New(frt, provq,
		batched.KeyProvider(init.KeyProviderFunc),
		batched.Datastore(ds),
	)
	if err != nil {
		return nil, xerrors.Errorf("setup batched provider: %w", err)
	}

	prov.Run() // TODO: call close at some point

	return &Node{
		Dht:        ipfsdht,
		FilDht:     fildht,
		FullRT:     frt,
		Provider:   prov,
		Host:       h,
		Blockstore: mbs,
		//Lmdb:       lmdbs,
		Datastore:  ds,
		Bitswap:    bswap.(*bitswap.Bitswap),
		Wallet:     wallet,
		Bwc:        bwc,
		Config:     cfg,
		StorageDir: stordir,
		Peering:    peerServ,
	}, nil
}

// Converting the public key to a multiaddress.
func toMultiAddress(addr string) (multiaddr.Multiaddr, error) {
	a, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse string multi addr: %w", err)
	}
	return a, nil
}

// It takes a slice of strings and returns a slice of multiaddresses
func toMultiAddresses(addrs []string) ([]multiaddr.Multiaddr, error) {
	var multiAddrs []multiaddr.Multiaddr
	for _, addr := range addrs {
		a, err := toMultiAddress(addr)
		if err != nil {
			log.Errorf("toMultiAddresses failed: %s", err)
		}
		multiAddrs = append(multiAddrs, a)
	}
	return multiAddrs, nil
}

func parseBsCfg(bscfg string) (string, []string, string, error) {
	if bscfg[0] != ':' {
		return "", nil, "", fmt.Errorf("cfg must start with colon")
	}

	var inParen bool
	var parenStart int
	var parenEnd int
	var end int
	for i := 1; i < len(bscfg); i++ {
		if inParen {
			if bscfg[i] == ')' {
				inParen = false
				parenEnd = i
			}
			continue
		}

		if bscfg[i] == '(' {
			inParen = true
			parenStart = i
		}

		if bscfg[i] == ':' {
			end = i
			break
		}
	}

	if parenStart == 0 {
		return bscfg[1:end], nil, bscfg[end+1:], nil
	}

	t := bscfg[1:parenStart]
	params := strings.Split(bscfg[parenStart+1:parenEnd], ",")

	return t, params, bscfg[end+1:], nil
}

/* format:
:lmdb:/path/to/thing
*/
func constructBlockstore(bscfg string) (EstuaryBlockstore, string, error) {
	if !strings.HasPrefix(bscfg, ":") {
		lmdbs, err := lmdb.Open(&lmdb.Options{
			Path:   bscfg,
			NoSync: true,
		})
		if err != nil {
			return nil, "", err
		}
		return lmdbs, bscfg, nil
	}

	spec, params, path, err := parseBsCfg(bscfg)
	if err != nil {
		return nil, "", err
	}

	switch spec {
	case "lmdb":
		lmdbs, err := lmdb.Open(&lmdb.Options{
			Path:   path,
			NoSync: true,
		})
		if err != nil {
			return nil, path, err
		}
		return lmdbs, "", nil
	case "flatfs":
		if len(params) > 0 {
			return nil, "", fmt.Errorf("flatfs params not yet supported")
		}
		sf, err := flatfs.ParseShardFunc("/repo/flatfs/shard/v1/next-to-last/3")
		if err != nil {
			return nil, "", err
		}

		ds, err := flatfs.CreateOrOpen(path, sf, false)
		if err != nil {
			return nil, "", err
		}

		return &deleteManyWrap{blockstore.NewBlockstoreNoPrefix(ds)}, path, nil
	case "migrate":
		if len(params) != 2 {
			return nil, "", fmt.Errorf("migrate blockstore requires two params (%d given)", len(params))
		}

		from, _, err := constructBlockstore(params[0])
		if err != nil {
			return nil, "", fmt.Errorf("failed to construct source blockstore for migration: %w", err)
		}

		to, destPath, err := constructBlockstore(params[1])
		if err != nil {
			return nil, "", fmt.Errorf("failed to construct dest blockstore for migration: %w", err)
		}

		mgbs, err := migratebs.NewBlockstore(from, to, true)
		if err != nil {
			return nil, "", err
		}

		return mgbs, destPath, nil
	default:
		return nil, "", fmt.Errorf("unrecognized blockstore spec: %q", spec)
	}
}

func loadBlockstore(bscfg string, wal string, flush, walTruncate, nocache bool) (blockstore.Blockstore, string, error) {
	bstore, dir, err := constructBlockstore(bscfg)
	if err != nil {
		return nil, "", err
	}

	if wal != "" {
		opts := badgerbs.DefaultOptions(wal)
		opts.Truncate = walTruncate

		writelog, err := badgerbs.Open(opts)
		if err != nil {
			return nil, "", err
		}

		ab, err := autobatch.NewBlockstore(bstore, writelog, 200, 200, flush)
		if err != nil {
			return nil, "", err
		}

		if flush {
			if err := ab.Flush(context.Background()); err != nil {
				return nil, "", err
			}
		}

		if walTruncate {
			return nil, "", fmt.Errorf("truncation and full flush complete, halting execution")
		}

		bstore = ab
	}

	ctx := metri.CtxScope(context.TODO(), "estuary.bstore")

	bstore = bsm.New("estuary.blks.base", bstore)

	if !nocache {
		cbstore, err := blockstore.CachedBlockstore(ctx, bstore, blockstore.CacheOpts{
			//HasBloomFilterSize:   512 << 20,
			//HasBloomFilterHashes: 7,
			HasARCCacheSize: 8 << 20,
		})
		if err != nil {
			return nil, "", err
		}
		bstore = &deleteManyWrap{cbstore}
	}

	notifbs := NewNotifBs(bstore)
	mbs := bsm.New("estuary.repo", notifbs)

	var blkst blockstore.Blockstore = mbs

	return blkst, dir, nil
}

func loadOrInitPeerKey(kf string) (crypto.PrivKey, error) {
	data, err := ioutil.ReadFile(kf)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		k, _, err := crypto.GenerateEd25519Key(crand.Reader)
		if err != nil {
			return nil, err
		}

		data, err := crypto.MarshalPrivateKey(k)
		if err != nil {
			return nil, err
		}

		if err := ioutil.WriteFile(kf, data, 0600); err != nil {
			return nil, err
		}

		return k, nil
	}
	return crypto.UnmarshalPrivateKey(data)
}

func setupWallet(dir string) (*wallet.LocalWallet, error) {
	kstore, err := keystore.OpenOrInitKeystore(dir)
	if err != nil {
		return nil, err
	}

	wallet, err := wallet.NewWallet(kstore)
	if err != nil {
		return nil, err
	}

	addrs, err := wallet.WalletList(context.TODO())
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		_, err := wallet.WalletNew(context.TODO(), types.KTSecp256k1)
		if err != nil {
			return nil, err
		}
	}

	defaddr, err := wallet.GetDefault()
	if err != nil {
		return nil, err
	}

	fmt.Println("Wallet address is: ", defaddr)

	return wallet, nil
}

type deleteManyWrap struct {
	blockstore.Blockstore
}

func (dmw *deleteManyWrap) DeleteMany(ctx context.Context, cids []cid.Cid) error {
	for _, c := range cids {
		if err := dmw.Blockstore.DeleteBlock(ctx, c); err != nil {
			return err
		}
	}

	return nil
}
