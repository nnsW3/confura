package mysql

import (
	"encoding/json"
	"strconv"

	"github.com/Conflux-Chain/go-conflux-sdk/types"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type transaction struct {
	ID     uint64
	Epoch  uint64 `gorm:"not null;index"`
	HashId uint64 `gorm:"not null;index"` // as an index, number is better than long string
	Hash   string `gorm:"size:66;not null"`
	// TODO maybe varchar(N) is enough, otherwise, query from other table or fullnode.
	// The same for other BLOB type data, e.g. log.Data.
	// However, if mysql engine automatically handle this case, just ignore this comment.
	TxRawData         []byte `gorm:"type:MEDIUMBLOB;not null"`
	TxRawDataLen      uint64 `gorm:"not null"`
	ReceiptRawData    []byte `gorm:"type:MEDIUMBLOB"`
	ReceiptRawDataLen uint64 `gorm:"not null"`
}

func (transaction) TableName() string {
	return "txs"
}

func hash2ShortId(hash string) uint64 {
	// first 8 bytes of hex string with 0x prefixed
	id, err := strconv.ParseUint(hash[2:18], 16, 64)
	if err != nil {
		logrus.WithError(err).WithField("hash", hash).Fatalf("Failed convert hash to short id")
	}

	return id
}

func newTx(tx *types.Transaction, receipt *types.TransactionReceipt) *transaction {
	result := &transaction{
		Epoch:          uint64(*receipt.EpochNumber),
		Hash:           tx.Hash.String(),
		TxRawData:      mustMarshalJSON(tx),
		ReceiptRawData: mustMarshalJSON(receipt),
	}

	result.HashId = hash2ShortId(result.Hash)
	result.TxRawDataLen = uint64(len(result.TxRawData))
	result.ReceiptRawDataLen = uint64(len(result.ReceiptRawData))

	return result
}

func loadTx(db *gorm.DB, txHash string) (*transaction, error) {
	hashId := hash2ShortId(txHash)

	var tx transaction

	db = db.Where("hash_id = ? AND hash = ?", hashId, txHash).First(&tx)
	if err := db.Scan(&tx).Error; err != nil {
		return nil, err
	}

	return &tx, nil
}

type block struct {
	ID         uint64
	Epoch      uint64 `gorm:"not null;index"`
	HashId     uint64 `gorm:"not null;index"`
	Hash       string `gorm:"size:66;not null"`
	Pivot      bool   `gorm:"not null"`
	RawData    []byte `gorm:"type:MEDIUMBLOB;not null"`
	RawDataLen uint64 `gorm:"not null"`
}

func newBlock(data *types.Block, pivot bool) *block {
	block := &block{
		Epoch:   data.EpochNumber.ToInt().Uint64(),
		Hash:    data.Hash.String(),
		Pivot:   pivot,
		RawData: mustMarshalJSON(block2Summary(data)),
	}

	block.HashId = hash2ShortId(block.Hash)
	block.RawDataLen = uint64(len(block.RawData))

	return block
}

func block2Summary(block *types.Block) *types.BlockSummary {
	summary := types.BlockSummary{
		BlockHeader:  block.BlockHeader,
		Transactions: make([]types.Hash, 0, len(block.Transactions)),
	}

	for _, tx := range block.Transactions {
		summary.Transactions = append(summary.Transactions, tx.Hash)
	}

	return &summary
}

func loadBlock(db *gorm.DB, whereClause string, args ...interface{}) (*types.BlockSummary, error) {
	var blk block

	db = db.Where(whereClause, args...).First(&blk)
	if err := db.Scan(&blk).Error; err != nil {
		return nil, err
	}

	var summary types.BlockSummary
	mustUnmarshalJSON(blk.RawData, &summary)

	return &summary, nil
}

type log struct {
	ID              uint64
	Epoch           uint64 `gorm:"not null;index"`
	BlockHash       string `gorm:"size:66;not null"`
	ContractAddress string `gorm:"size:64;not null"`
	Topic0          string `gorm:"size:66;not null"`
	Topic1          string `gorm:"size:66"`
	Topic2          string `gorm:"size:66"`
	Topic3          string `gorm:"size:66"`
	Data            []byte `gorm:"type:MEDIUMBLOB"`
	DataLen         uint64 `gorm:"not null"`
	TxHash          string `gorm:"size:66;not null"`
	TxIndex         uint64 `gorm:"not null"`
	TxLogIndex      uint64 `gorm:"not null"`
	LogIndex        uint64 `gorm:"not null"`
}

func newLog(data *types.Log) *log {
	log := &log{
		Epoch:           data.EpochNumber.ToInt().Uint64(),
		BlockHash:       data.BlockHash.String(),
		ContractAddress: data.Address.MustGetBase32Address(),
		Data:            []byte(data.Data),
		Topic0:          data.Topics[0].String(),
		TxHash:          data.TransactionHash.String(),
		TxIndex:         data.TransactionIndex.ToInt().Uint64(),
		TxLogIndex:      data.TransactionLogIndex.ToInt().Uint64(),
		LogIndex:        data.LogIndex.ToInt().Uint64(),
	}

	log.DataLen = uint64(len(log.Data))

	numTopics := len(data.Topics)

	if numTopics > 1 {
		log.Topic1 = data.Topics[1].String()
	}

	if numTopics > 2 {
		log.Topic2 = data.Topics[2].String()
	}

	if numTopics > 3 {
		log.Topic3 = data.Topics[3].String()
	}

	return log
}

func mustMarshalJSON(v interface{}) []byte {
	if v == nil {
		return nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		logrus.WithError(err).Fatalf("Failed to marshal data to JSON, value = %+v", v)
	}

	return data
}

func mustUnmarshalJSON(data []byte, v interface{}) {
	if err := json.Unmarshal(data, v); err != nil {
		logrus.WithError(err).Fatalf("Failed to unmarshal data, data = %v", string(data))
	}
}
