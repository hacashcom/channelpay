package chanpay

import (
	"fmt"
	"github.com/hacash/channelpay/protocol"
	"github.com/hacash/core/channel"
	"github.com/hacash/core/fields"
	"github.com/hacash/node/websocket"
	"sync/atomic"
)

/**
 * 通道方
 */

// 通道连接方
type ChannelSideConn struct {

	// ws长连接
	WsConn *websocket.Conn

	// 数据
	ChannelId   fields.Bytes16               // 通道链 ID
	ChannelInfo *protocol.RpcDataChannelInfo // 通道当前的信息

	OurAddress    fields.Address // 我方地址
	RemoteAddress fields.Address // 对方地址（客户地址或结算通道对方地址）

	// 最新的对账票据
	LatestReconciliationBalanceBill channel.ReconciliationBalanceBill

	// 支付收款状态锁 0:未占用  1:占用状态
	businessExclusiveStatus uint32 //

}

func NewChannelSideConn(conn *websocket.Conn) *ChannelSideConn {
	return &ChannelSideConn{
		WsConn:                          conn,
		ChannelInfo:                     nil,
		LatestReconciliationBalanceBill: nil,
		businessExclusiveStatus:         0,
	}
}

func (c *ChannelSideConn) SetChannelId(id fields.Bytes16) {
	c.ChannelId = id
}

func (c *ChannelSideConn) GetChannelId() fields.Bytes16 {
	return c.ChannelId
}

func (c *ChannelSideConn) SetChannelInfo(info *protocol.RpcDataChannelInfo) {
	c.ChannelInfo = info
}

func (c *ChannelSideConn) GetChannelInfo() *protocol.RpcDataChannelInfo {
	return c.ChannelInfo
}

func (c *ChannelSideConn) SetAddresses(our, remote fields.Address) {
	c.OurAddress = our
	c.RemoteAddress = remote
}

func (c *ChannelSideConn) GetOurAddress() fields.Address {
	return c.OurAddress
}

func (c *ChannelSideConn) GetRemoteAddress() fields.Address {
	return c.RemoteAddress
}

func (c *ChannelSideConn) SetReconciliationBill(bill channel.ReconciliationBalanceBill) {
	c.LatestReconciliationBalanceBill = bill
}

func (c *ChannelSideConn) GetReconciliationBill() channel.ReconciliationBalanceBill {
	return c.LatestReconciliationBalanceBill
}

// 检查收款通道是否被占用
func (c *ChannelSideConn) IsInBusinessExclusive() bool {
	// 检查状态
	return atomic.LoadUint32(&c.businessExclusiveStatus) == 1
}

// 其中状态独占
func (c *ChannelSideConn) StartBusinessExclusive() bool {
	return atomic.CompareAndSwapUint32(&c.businessExclusiveStatus, 0, 1)
}

// 解除状态独占
func (c *ChannelSideConn) ClearBusinessExclusive() {
	atomic.CompareAndSwapUint32(&c.businessExclusiveStatus, 1, 0)
}

// 判断
func (c *ChannelSideConn) RemoteAddressIsLeft() bool {
	return c.RemoteAddress.Equal(c.ChannelInfo.LeftAddress)
}

// 获取通道容量
// side = our, remote
func (c *ChannelSideConn) GetChannelCapacityAmount(side string) fields.Amount {
	leftAmt := c.ChannelInfo.LeftAmount
	rightAmt := c.ChannelInfo.RightAmount
	// 判断是否有收据
	bill := c.LatestReconciliationBalanceBill
	if bill != nil {
		leftAmt = bill.GetLeftBalance()
		rightAmt = bill.GetRightBalance()
	}
	remoteIsLeft := c.RemoteAddress.Equal(c.ChannelInfo.LeftAddress)
	// 返回容量
	if (side == "remote" && remoteIsLeft) ||
		(side == "our" && !remoteIsLeft) {
		return leftAmt
	} else {
		return rightAmt
	}

}
func (c *ChannelSideConn) GetChannelCapacityAmountOfOur() fields.Amount {
	return c.GetChannelCapacityAmount("our")
}
func (c *ChannelSideConn) GetChannelCapacityAmountOfRemote() fields.Amount {
	return c.GetChannelCapacityAmount("remote")
}

// 创建对账单
func (c *ChannelSideConn) CreateNewProveBodyByDoPayFromSide(side string, payamt *fields.Amount) (*channel.ChannelChainTransferProveBodyInfo, error) {

	if side != "our" && side != "remote" {
		return nil, fmt.Errorf("side %s error", side)
	}

	// 检查容量
	amtcap := c.GetChannelCapacityAmount(side)
	if amtcap.LessThan(payamt) {
		return nil, fmt.Errorf("%s side channel capacity balance not enough.", side)
	}

	// 创建
	body := &channel.ChannelChainTransferProveBodyInfo{}

	return body, nil
}

// 直接保存（不做检查）支付对账票据
func (c *ChannelSideConn) UncheckSignSaveChannelPayReconciliationBalanceBill(bills *channel.ChannelPayCompleteDocuments) error {

	// 找出对账单
	var proveBody *channel.ChannelChainTransferProveBodyInfo = nil
	for _, v := range bills.ProveBodys.ProveBodys {
		if v.ChannelId.Equal(c.ChannelId) {
			proveBody = v
			break
		}
	}
	// 是否存在
	if proveBody == nil {
		return fmt.Errorf("proveBody of channel id %s not find", c.ChannelId.ToHex())
	}
	// 检查对账流水号
	if c.ChannelInfo.ReuseVersion != proveBody.ReuseVersion {
		return fmt.Errorf("ReuseVersion not match need %d but got %d", c.ChannelInfo.ReuseVersion, proveBody.ReuseVersion)
	}
	needBillAutoNumber := fields.VarUint8(1)
	if c.LatestReconciliationBalanceBill != nil {
		needBillAutoNumber = fields.VarUint8(c.LatestReconciliationBalanceBill.GetAutoNumber() + 1)
	}
	if needBillAutoNumber != proveBody.BillAutoNumber {
		return fmt.Errorf("BillAutoNumber not match need %d but got %d", needBillAutoNumber, proveBody.BillAutoNumber)
	}

	// 保存
	c.LatestReconciliationBalanceBill = &channel.OffChainCrossNodeSimplePaymentReconciliationBill{
		ChannelChainTransferTargetProveBody: *proveBody,
		ChannelChainTransferData:            *bills.ChainPayment,
	}

	// 成功
	return nil
}

/////////////////////////////////////////////////////

type ChannelSideConnWrap interface {
	GetChannelCapacityAmountForRemoteCollect() fields.Amount
}

// 按通道容量排序
type ChannelSideConnListByCollectCapacity []ChannelSideConnWrap

func (c ChannelSideConnListByCollectCapacity) Len() int {
	return len(c)
}

func (n ChannelSideConnListByCollectCapacity) Less(i, j int) bool {
	//fmt.Println(i, j, n[i] < n[j], n)
	jamt := n[j].GetChannelCapacityAmountForRemoteCollect()
	if n[i].GetChannelCapacityAmountForRemoteCollect().LessThan(&jamt) {
		return true
	} else {
		return false
	}
}

func (n ChannelSideConnListByCollectCapacity) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}