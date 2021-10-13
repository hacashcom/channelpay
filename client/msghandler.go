package client

import (
	"fmt"
	"github.com/hacash/channelpay/protocol"
	"strings"
)

/**
 * 消息处理
 */
func (c *ChannelPayClient) startMsgHandler() {
	// 订阅消息处理，
	chanobj := make(chan protocol.Message, 1)
	subObj := c.user.servicerStreamSide.ChannelSide.SubscribeMessage(chanobj)
	c.user.msgSubObj = subObj
	// 循环处理消息
	go func() {
		//defer fmt.Println("ChannelPayUser.MsgHandler end")
		for {
			select {
			case v := <-chanobj:
				go c.dealMsg(v)
			case <-subObj.Err():
				c.logoutConnectWindowShow("Network exception. You have logged out") // 退出
				return                                                              // 订阅关闭
			}
		}
	}()
}

// 消息处理
func (c *ChannelPayClient) dealMsg(msg protocol.Message) {
	ty := msg.Type()
	switch ty {
	// 发起收款
	case protocol.MsgTypeInitiatePayment:
		msgobj := msg.(*protocol.MsgRequestInitiatePayment)
		c.dealInitiatePayment(msgobj)

	// 支付预查询返回
	case protocol.MsgTypeResponsePrequeryPayment:
		msgobj := msg.(*protocol.MsgResponsePrequeryPayment)
		if msgobj.ErrCode > 0 {
			// 错误显示
			c.ShowPaymentErrorString("Prequery payment error: " + msgobj.ErrTip.Value())
			return
		}
		// 调用前端界面，开始支付
		//fmt.Println("PayPathCount: ", msgobj.PathForms.PayPathCount)
		c.dealPrequeryPaymentResult(msgobj)

	// 被顶下线
	case protocol.MsgTypeDisplacementOffline:
		c.logoutConnectWindowShow("You have login at another place and this connection has been exited") // 退出
	}

}

// 退出登录
func (c *ChannelPayClient) logoutConnectWindowShow(tip string) {
	c.user.Logout() // 退出登录
	// 向界面发出退出登录消息
	lgv := c.payui.Eval(fmt.Sprintf(`Logout("%s")`, strings.Replace(tip, `"`, ``, -1))) // 退出
	if DevDebug {
		fmt.Println("Logout() => ", tip, lgv.String())
	}
}