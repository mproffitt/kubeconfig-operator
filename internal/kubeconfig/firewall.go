package kubeconfig

import "fmt"

func (m *Manager) makeFirewallRule(localIp, publicIp, port string) string {
	switch m.cluster.Spec.FirewallFormat {
	case "nftables":
		return fmt.Sprintf(
			"nft add rule ip nat prerouting ip daddr %s tcp dport %s dnat to %s:%s",
			publicIp, port, localIp, port,
		)
	case "ufw":
		return fmt.Sprintf(
			"ufw route allow proto tcp from any to %s port %s comment 'DNAT to %s:%s'",
			publicIp, port, localIp, port,
		)
	case "firewalld":
		return fmt.Sprintf(
			"firewall-cmd --zone=public --add-rich-rule='rule family=\"ipv4\" "+
				"forward-port port=\"%s\" protocol=\"tcp\" to-addr=\"%s\" to-port=\"%s\"'",
			port, localIp, port,
		)
	case "ipfw":
		return fmt.Sprintf(
			"ipfw add 100 fwd %s,%s tcp from any to %s %s",
			localIp, port, publicIp, port,
		)
	case "pf":
		return fmt.Sprintf(
			"rdr pass on egress proto tcp from any to %s port %s -> %s port %s",
			publicIp, port, localIp, port,
		)
	default:
		return fmt.Sprintf(
			"iptables -t nat -A PREROUTING -p tcp -d %s --dport %s -j DNAT --to-destination %s:%s",
			publicIp, port, localIp, port,
		)
	}
}

func (m *Manager) makeDeleteFirewallRule(localIp, publicIp, port string) string {
	switch m.cluster.Spec.FirewallFormat {
	case "nftables":
		return fmt.Sprintf(
			"nft delete rule ip nat prerouting ip daddr %s tcp dport %s dnat to %s:%s",
			publicIp, port, localIp, port,
		)
	case "ufw":
		return fmt.Sprintf(
			"ufw route delete allow proto tcp from any to %s port %s",
			publicIp, port,
		)
	case "firewalld":
		return fmt.Sprintf(
			"firewall-cmd --zone=public --remove-rich-rule='rule family=\"ipv4\" "+
				"forward-port port=\"%s\" protocol=\"tcp\" to-addr=\"%s\" to-port=\"%s\"'",
			port, localIp, port,
		)
	case "ipfw":
		return "ipfw delete 100"
	case "pf":
		return fmt.Sprintf(
			"no rdr pass on egress proto tcp from any to %s port %s",
			publicIp, port,
		)
	default:
		return fmt.Sprintf(
			"iptables -t nat -D PREROUTING -p tcp -d %s --dport %s -j DNAT --to-destination %s:%s",
			publicIp, port, localIp, port,
		)
	}
}
