# Vert
个人兴趣使然的WebServer，支持HTTP/2、TLS 1.3，支持自动签发SSL证书（Let's Encrypt），可以作为静态文件Server，亦可作为反向代理（支持上游HTTP与WebSocket）。

推荐使用Go 1.13或更高版本进行编译。
## 使用方法

    ./Vert --conf conf.yaml

## 完整配置示例&说明

配置文件为yaml格式。

    base: # 基本配置
      log_level: debug # 日志等级，可选项： debug info error
      log_file: /path/to/log/file # 日志文件路径，会自动在文件尾部添加 .YYYYMMDD 的后缀。
      tls_email: xxx@example.com # 可选字段，HTTPS证书在签发时登记的邮件地址，用于接收证书更新情况。
      cert_cache: /path/to/cert/cache_dir # 可选字段，自动签发的证书的缓存目录，建议配置。
    upstream: # 反代上游配置
      upstream_1: # 上游名称
        - 10.1.1.1:12345
        - 10.1.1.2:12345 weight=3 # 可以配置 round_robin 的轮询权重
      upstream_2 round_robin: # 上游名称后面可以配置反代选择策略
        - 10.2.2.1:54321 weight=2
        - 10.2.2.2:54321
    sites: # 网站配置
      www.example.com: # 域名
        - type: http # 网站协议，可选项： http https，若无配置则根据端口进行猜测
          port: 80 # 监听端口，若无配置，则根据type自动设置
          rules: # 路由规则表
            - /: # path匹配前缀
              - 'redirect https://{host}{path}{has_query}{query}{has_fragment}{fragment}'
        - type: https
          port: 443
          autocert: true # 是否使用自动签发证书，设置为true则忽略ssl_key & ssl_cert
          ssl_key: /path/to/ssl/key_file # SSL证书私钥文件
          ssl_cert: /path/to/ssl/cert_file # SSL证书文件（fullchain）
          rules:
            - /1/:
              - 'proxy http://{up:upstream_1}/{seg[1:]}{has_query}{query}{has_fragment}{fragment}'
            - /2/:
              - 'proxy ws://{up:upstream_2}/{seg[1:]}{has_query}{query}{has_fragment}{fragment}'
            - /3/:
              - 'set-header Host backend3.com'
              - 'proxy-cookie {host} .backend3.com'
              - 'proxy wss://backend3.com/{seg[1:]}{has_query}{query}{has_fragment}{fragment}'
            - /:
              - wwwroot /path/to/www/html
      www.site2.com:
        # ...

## 反代上游配置格式

    NAME [STRATEGY]:
      - ADDRESS_1 [weight=N]
      - ...

上游名称后面可以指定使用的路由选择策略：

- round_robin：默认配置，加权轮询。
- random：随机选择。
- client_hash：根据客户端IP进行哈希，对同样的IP会选择固定的后端地址。

地址后可以添加`weight=N`的可选项，用于指定`round_robin`的权重，默认权重为1。

## 路由规则表

每个路由规则表由多个前缀匹配规则组成，从上到下匹配PATH前缀。

每个前缀下又可以配置一系列的`动作`，动作也是从上到下执行，可以进行各种操作。

动作的参数支持使用大括号括起的变量，关于变量，下文再详细叙述。

动作分为三种类型：

1. 修改原始请求包
2. 修改反代上游回包
3. 最终动作

配置多条动作时需要遵循一个原则：

`修改反代上游回包`下方的动作只能是`最终动作`，或者另一个`修改反代上游回包`动作。（因为我懒所以有了这个限制）

### 修改原始请求包

    set-header HeaderName Value

设置原始请求包的HTTP Header，Value的内容可以使用变量。

    del-header HeaderName

从原始请求包的HTTP Header中删除指定字段。

    limit-referer Value

对`PATH=/`以外的请求，检查Referer是否为指定的Value，如果通不过检测则直接返回403 Forbidden，Value可以使用变量。
### 修改反代上游回包

    set-rsp-header HeaderName Value

设置反代上游回包的HTTP Header，Value的内容可以使用变量。

    del-rsp-header HeaderName

从反代上游回包的HTTP Header中删除指定字段。

    proxy-cookie HostLocal HostUpstream

将反代上游回包的`Set-Cookie`中的`domain=HostUpstream`改为`domain=HostLocal`，HostLocal和HostUpstream都可以使用变量。

    filter-content REGEXP Replacement

对上游回包的Body，使用`REGEXP`进行正则匹配（正则表达式语法同[re2库](https://github.com/google/re2/wiki/Syntax)），并将匹配到的内容替换为`Replacement`（支持使用变量）。

### 最终动作

    redirect TargetAddress

进行HTTP 301跳转，TargetAddress支持使用变量。

    wwwroot /path/to/www/html

指定静态文件服务根目录。

    proxy TargetAddress

反向代理，TargetAddress支持使用变量。

TargetAddress的协议支持http、https、ws、wss，后两种用于对WebSocket进行反向代理（wss=ws+tls）。

## 变量

在Vert的动作规则中，可以使用一系列的变量，动态生成动作的参数。

变量由`{}`大括号括起，当大括号内部由一个`%`开头时（即`{%...}`格式），表示对变量的值进行URL Encode。

Vert有这些变量：

### `{path}`

原始请求的完整PATH，带有`/`前缀。

示例：原始请求PATH=/hello/world

`http://www.example.com{path}` ==> `http://www.example.com/hello/world`

### `{path[N:M]}`

取原始请求的PATH的子串，下标范围`[N,M)`。下标从0开始，N与M均为非负整数。

N与M均可省略，N省略时表示从开头开始，M省略时表示到结束为止。（即`{path[:]}`与`{path}`等效）

示例：原始请求PATH=/hello/world

`http://www.example.com{path[6:]}` ==> `http://www.example.com/world`

### `{seg[N]}`

将原始请求的PATH，根据`/`进行分段，并取第N段（不带`/`前缀），N从0开始。

示例：原始请求PATH=/hello/world

`http://www.example.com/{seg[0]}` ==> `http://www.example.com/hello`

### `seg[N,M]`

将原始请求的PATH，根据`/`进行分段，并取`[N,M)`区间，多段之间自动用`/`分隔。

N与M均可省略，N省略时表示从开头开始，M省略时表示到结束为止。

示例：原始请求PATH=/hello/world/hehehe/xxxxxx

`http://www.example.com/{seg[1:3]}` ==> `http://www.example.com/world/hehehe`

### `{host}`

原始请求Header中的Host字段值，此变量不受`set-header Host XXX`影响，永远返回最初的Host（即配置文件中指定的域名）。

### `{has_query}`

若原始请求包含`?key1=value1&key2=value2`的查询部分，则此变量值为`?`，否则为空串。

### `{query}`

原始请求的完整query。

示例：原始请求URL=`http://www.example.com/?a=1&b=2&c=3`

`{query}` ==> `a=1&b=2&c=3`

注意这里没有带上`?`前缀。

### `{query:XXX}`

取原始请求的query中指定key的值，会自动被URL Decode。如果原始query不包含指定key，则为空串。

示例：原始请求query=`a=1&b=2&c=%2a%26%25`

`{query:c}` ==> `*&%`

`{%query:c}` ==> `%2A%26%25`

这里顺便演示一下大括号内加`%`开头的效果。

### `{query:[YYY,YYY,ZZZ]}`

取原始请求的query中指定的多个key的值，并且自动重新组合成query格式。

示例：原始请求query=`a=1&b=2&c=%2a%26%25`

`{query:[a,c]}` ==> `a=1&c=%2a%26%25`

### `{^query:[YYY,YYY,ZZZ]}`

相当于上一个变量“取反”。

示例：原始请求query=`a=1&b=2&c=%2a%26%25`

`{^query:[a,c]}` ==> `b=2`

### `{has_fragment}`

若原始请求URL末尾包含`#XXXXXX`的fragment部分，则此变量值为`#`，否则为空串。

### `{fragment}`

原始请求末尾的完整fragment，不包含`#`前缀。

### `{up:XXX}`

查询指定的上游名称的实际地址。名称未配置则为空串。

### `{re[N]}`

专门用于`filter-content`的`Replacement`部分的变量（`Replacement`亦可使用上述其他变量），N表示正则表达式匹配到的第N个“子串”，N从**1**开始。