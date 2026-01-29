# Bandwidth Benchmarks: Single vs Multi Mode (v0.5.0)

This document presents benchmark results for dnstm v0.5.0, comparing single-tunnel mode versus multi-tunnel mode (dnsrouter) for different tunnel types.

## Test Configuration

- **Test File:** 10MB (speedtest.tele2.net/10MB.zip)
- **Resolver:** 1.1.1.1:53 (Cloudflare)
- **Server:** test-server (203.0.113.50)
- **Client:** Local machine via SOCKS5 proxy
- **Date:** 2026-01-29
- **Runs:** 5 rounds per configuration
- **MTU:** 900 for all transports (see note below)

### MTU Configuration

DNSTT MTU is set to 900 to match [Slipstream's hard-coded MTU of 900](https://github.com/Mygod/slipstream-rust/blob/af38c2b4ca762b75b2c08d79632b8f6ef5208806/docs/config.md?plain=1#L41) for fair comparison. DNSTT's default MTU is 1232, but using different MTU values would make protocol comparison unfair.

### Modes Tested

| Mode       | Description                                                              |
| ---------- | ------------------------------------------------------------------------ |
| **Single** | Active transport binds directly to EXTERNAL_IP:53                        |
| **Multi**  | DNS router on EXTERNAL_IP:53 forwards to transport services on localhost |

## Results

### Slipstream SOCKS (slip1)

Uses DNS-over-QUIC protocol for tunneling with microsocks target.

| Mode   | Run 1  | Run 2  | Run 3  | Run 4  | Run 5  | **Avg (B/s)** |
| ------ | ------ | ------ | ------ | ------ | ------ | ------------- |
| Single | 66,795 | 65,866 | 57,663 | 63,777 | 61,742 | **63,169**    |
| Multi  | 57,500 | 68,843 | 59,464 | 62,992 | 57,769 | **61,314**    |

**Router Overhead: 3%**

### DNSTT SOCKS (dnstt1)

Uses DNS TXT record encoding for tunneling with microsocks target.

| Mode   | Run 1  | Run 2  | Run 3  | Run 4  | Run 5  | **Avg (B/s)** |
| ------ | ------ | ------ | ------ | ------ | ------ | ------------- |
| Single | 40,310 | 43,132 | 41,502 | 44,579 | 42,502 | **42,405**    |
| Multi  | 40,994 | 46,047 | 44,045 | 45,452 | 43,719 | **44,051**    |

**Router Overhead: -4%** (multi slightly faster)

### Slipstream + Shadowsocks (ss1)

Shadowsocks encryption over Slipstream DNS-over-QUIC tunnel.

| Mode   | Run 1  | Run 2  | Run 3  | Run 4  | Run 5  | **Avg (B/s)** |
| ------ | ------ | ------ | ------ | ------ | ------ | ------------- |
| Single | 57,733 | 65,916 | 59,157 | 57,572 | 65,323 | **61,140**    |
| Multi  | 57,267 | 65,194 | 59,825 | 57,598 | 64,818 | **60,940**    |

**Router Overhead: <1%**

## Summary

| Transport                | Single Mode | Multi Mode | Overhead |
| ------------------------ | ----------- | ---------- | -------- |
| Slipstream SOCKS         | 63.2 KB/s   | 61.3 KB/s  | -3%      |
| DNSTT SOCKS              | 42.4 KB/s   | 44.1 KB/s  | -4%      |
| Slipstream + Shadowsocks | 61.1 KB/s   | 60.9 KB/s  | <1%      |

### Key Observations

1. **Slipstream outperforms DNSTT** (~1.5x faster with equal MTU)
2. **Multi-mode overhead is minimal** (1-4%)
3. **Shadowsocks encryption adds ~3% overhead** to Slipstream

### Protocol Comparison

| Protocol   | Single Mode | Notes                            |
| ---------- | ----------- | -------------------------------- |
| Slipstream | 63.2 KB/s   | Best performance                 |
| SS over SL | 61.1 KB/s   | Encryption adds minimal overhead |
| DNSTT      | 42.4 KB/s   | Consistent, reliable             |

Slipstream is approximately **1.5x faster** than DNSTT under current network conditions.

## Analysis

## Variance Analysis

| Transport   | Min    | Max    | Variance |
| ----------- | ------ | ------ | -------- |
| Slip-SOCKS  | 57,500 | 68,843 | ±9%      |
| DNSTT-SOCKS | 40,310 | 46,047 | ±7%      |
| SS          | 57,267 | 65,916 | ±7%      |

## Recommendations

## When to Use Each Mode

**Single Mode:**

- You only need one active tunnel at a time
- Slightly better performance (1-4%)
- Simpler debugging

**Multi Mode:**

- You need multiple tunnels simultaneously
- Domain-based routing required
- Performance impact is minimal

## Raw Data (bytes/sec)

### Slipstream SOCKS Single

```
Run 1: 66795
Run 2: 65866
Run 3: 57663
Run 4: 63777
Run 5: 61742
Avg:   63169
```

### Slipstream SOCKS Multi

```
Run 1: 57500
Run 2: 68843
Run 3: 59464
Run 4: 62992
Run 5: 57769
Avg:   61314
```

### DNSTT SOCKS Single

```
Run 1: 40310
Run 2: 43132
Run 3: 41502
Run 4: 44579
Run 5: 42502
Avg:   42405
```

### DNSTT SOCKS Multi

```
Run 1: 40994
Run 2: 46047
Run 3: 44045
Run 4: 45452
Run 5: 43719
Avg:   44051
```

### Shadowsocks Single

```
Run 1: 57733
Run 2: 65916
Run 3: 59157
Run 4: 57572
Run 5: 65323
Avg:   61140
```

### Shadowsocks Multi

```
Run 1: 57267
Run 2: 65194
Run 3: 59825
Run 4: 57598
Run 5: 64818
Avg:   60940
```

---

# Authoritative/Direct Mode Benchmarks

Testing with direct connection to server IP (bypassing recursive resolvers).

## Test Configuration (Authoritative)

- **Test File:** 10MB (speedtest.tele2.net/10MB.zip)
- **Connection:** Direct to server IP (203.0.113.50:53)
- **Server:** test-server (203.0.113.50)
- **Client:** Local machine via SOCKS5 proxy
- **Date:** 2026-01-29
- **Runs:** 5 rounds per configuration
- **MTU:** 900 for all transports

### Connection Modes

| Transport   | Client Flag                        |
| ----------- | ---------------------------------- |
| Slipstream  | `--authoritative 203.0.113.50:53`  |
| DNSTT       | `-udp 203.0.113.50:53`             |
| Shadowsocks | plugin-opts: `authoritative=IP:53` |

## Results (Authoritative Mode)

### Slipstream SOCKS (slip1)

| Mode   | Run 1     | Run 2     | Run 3     | Run 4     | Run 5     | **Avg (B/s)** |
| ------ | --------- | --------- | --------- | --------- | --------- | ------------- |
| Single | 5,278,470 | 3,659,696 | 3,946,014 | 4,026,772 | 2,677,876 | **3,917,766** |
| Multi  | 3,876,366 | 2,966,061 | 2,652,097 | 554,043   | 552,880   | **2,120,289** |

**Router Overhead: 46%**

### DNSTT SOCKS (dnstt1)

| Mode   | Run 1   | Run 2   | Run 3   | Run 4   | Run 5   | **Avg (B/s)** |
| ------ | ------- | ------- | ------- | ------- | ------- | ------------- |
| Single | 748,446 | 738,814 | 747,041 | 748,704 | 757,704 | **748,142**   |
| Multi  | 750,425 | 744,067 | 747,131 | 728,723 | 755,185 | **745,106**   |

**Router Overhead: <1%**

### Slipstream + Shadowsocks (ss1)

| Mode   | Run 1     | Run 2     | Run 3     | Run 4     | Run 5     | **Avg (B/s)** |
| ------ | --------- | --------- | --------- | --------- | --------- | ------------- |
| Single | 3,427,418 | 2,428,175 | 2,512,737 | 2,611,872 | 2,652,406 | **2,726,522** |
| Multi  | 2,431,489 | 3,347,296 | 2,963,229 | 670,975   | 401,200   | **1,962,838** |

**Router Overhead: 28%**

## Summary (Authoritative Mode)

| Transport                | Single Mode   | Multi Mode | Overhead |
| ------------------------ | ------------- | ---------- | -------- |
| Slipstream SOCKS         | **3.92 MB/s** | 2.12 MB/s  | +46%     |
| DNSTT SOCKS              | 0.75 MB/s     | 0.75 MB/s  | <1%      |
| Slipstream + Shadowsocks | **2.73 MB/s** | 1.96 MB/s  | +28%     |

### Key Observations (Authoritative)

1. **Authoritative mode is ~18-90x faster** than recursive resolver mode
2. **Slipstream SOCKS achieves the highest throughput** (3.92 MB/s single mode)
3. **Slipstream outperforms DNSTT** (3.92 MB/s vs 0.75 MB/s, ~5x faster)
4. **DNSTT is very consistent** with minimal variance between runs
5. **Slipstream shows high variance** (0.55-5.28 MB/s range)
6. **Router overhead is significant for Slipstream** (28-46%)

### Protocol Comparison (Authoritative)

| Protocol   | Best Mode | Speed     | Notes                     |
| ---------- | --------- | --------- | ------------------------- |
| Slipstream | Single    | 3.92 MB/s | Fastest, high variance    |
| SS over SL | Single    | 2.73 MB/s | Encryption overhead ~30%  |
| DNSTT      | Either    | 0.75 MB/s | Most consistent, reliable |

**Slipstream is ~5x faster** than DNSTT in authoritative mode.

### Variance Analysis (Authoritative)

| Transport   | Min     | Max       | Variance |
| ----------- | ------- | --------- | -------- |
| Slip-SOCKS  | 552,880 | 5,278,470 | ±81%     |
| DNSTT-SOCKS | 728,723 | 757,704   | ±2%      |
| SS          | 401,200 | 3,427,418 | ±79%     |

## Comparison: Recursive vs Authoritative

| Transport   | Recursive | Authoritative | Speedup |
| ----------- | --------- | ------------- | ------- |
| Slip-SOCKS  | 63 KB/s   | 3.92 MB/s     | **62x** |
| DNSTT-SOCKS | 42 KB/s   | 0.75 MB/s     | **18x** |
| SS          | 61 KB/s   | 2.73 MB/s     | **45x** |

### Why Authoritative Mode is Faster

1. **No resolver latency** - Direct UDP to server
2. **No rate limiting** - Recursive resolvers may throttle
3. **No caching issues** - Direct connection avoids resolver caching
4. **Lower RTT** - Single hop vs multiple hops

## Recommendations

| Use Case                    | Recommended Setup                                 |
| --------------------------- | ------------------------------------------------- |
| Maximum bandwidth           | **Slipstream SOCKS + Authoritative + Single**     |
| Encrypted traffic           | **SS + Authoritative + Single mode**              |
| Censorship bypass (stealth) | **Slipstream + Recursive resolver**               |
| Reliable, consistent speed  | **DNSTT + Authoritative**                         |
| Multiple domains needed     | **Multi-mode** (28-46% overhead in authoritative) |

### When to Use Each Connection Mode

**Authoritative Mode:**

- Direct connection to your server is possible
- Maximum bandwidth required
- Lower latency needed
- Less stealth required

**Recursive Resolver Mode:**

- Need to hide server IP
- Censorship environment
- Stealth is priority over speed

## Raw Data - Authoritative Mode (bytes/sec)

### Slipstream SOCKS Single (Authoritative)

```
Run 1: 5278470
Run 2: 3659696
Run 3: 3946014
Run 4: 4026772
Run 5: 2677876
Avg:   3917766
```

### Slipstream SOCKS Multi (Authoritative)

```
Run 1: 3876366
Run 2: 2966061
Run 3: 2652097
Run 4: 554043
Run 5: 552880
Avg:   2120289
```

### DNSTT SOCKS Single (Direct)

```
Run 1: 748446
Run 2: 738814
Run 3: 747041
Run 4: 748704
Run 5: 757704
Avg:   748142
```

### DNSTT SOCKS Multi (Direct)

```
Run 1: 750425
Run 2: 744067
Run 3: 747131
Run 4: 728723
Run 5: 755185
Avg:   745106
```

### Shadowsocks Single (Authoritative)

```
Run 1: 3427418
Run 2: 2428175
Run 3: 2512737
Run 4: 2611872
Run 5: 2652406
Avg:   2726522
```

### Shadowsocks Multi (Authoritative)

```
Run 1: 2431489
Run 2: 3347296
Run 3: 2963229
Run 4: 670975
Run 5: 401200
Avg:   1962838
```

## Notes

- All speeds in bytes/sec unless otherwise noted
- Authoritative/direct mode bypasses recursive resolvers
- MTU set to 900 for all transports to match [Slipstream's hard-coded MTU](https://github.com/Mygod/slipstream-rust/blob/af38c2b4ca762b75b2c08d79632b8f6ef5208806/docs/config.md?plain=1#L41)
- High variance in Slipstream tests may be due to QUIC congestion control
- DNSTT shows most consistent performance across all tests
- Shadowsocks multi-mode had outlier runs (670K, 401K) affecting average
