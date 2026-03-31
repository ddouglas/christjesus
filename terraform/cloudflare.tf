

resource "cloudflare_zone" "bodyofchrist" {
  account = {
    id = var.cloudflare_account_id
  }
  name = "bodyofchrist.app"
  type = "full"
}

locals {
  boc = {
    mx = {
      "mx_10" = {
        name     = "bodyofchrist.app"
        priority = 10
        content  = "mx.zoho.com"
      }
      "mx_20" = {
        name     = "bodyofchrist.app"
        priority = 20
        content  = "mx2.zoho.com"
      }
      "mx_50" = {
        name     = "bodyofchrist.app"
        priority = 50
        content  = "mx3.zoho.com"
      }
    }
    txt = {
      "txt_verification" = {
        name    = "@"
        content = "zoho-verification=zb01327960.zmverify.zoho.com"
      }
      "txt_spf" = {
        name    = "@"
        content = "v=spf1 include:zohomail.com ~all"
      }
      "txt_dkim" = {
        name    = "first._domainkey"
        content = "v=DKIM1; k=rsa; p=MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA+gdy9eEaVUwf8AtWqdyuwHByRJZQEnXbuynKbvk6OPfmwcVCKnLO5WKs8+KgmmxFe12DMtDdRV/KndZ9adWx1+RNJaHziG5TQUryMRcMWLIKlfugz8NCTw9gehY5/R9eCPMDlDl1AGk1Kl3TC7dUatgUjCKWnsE7/mex3PhOdVF/FG0b4e3R/eZqWlhR56LTfHmLOG3+YbBYkZe1JOK5YoWKyrfZE36Jp2aB+efaff/z3nfvP5xzNmB8dqydYCh9DBEMAh/cfUVEdQteDL3GQSNF9DbeEM2/ITnfeRxZQmYxPHBDMbVtAlCndh5I4sOFir+vBRtXUVclW59gNw+GXwIDAQAB"
      }
      "txt_dmarc" = {
        name    = "_dmarc"
        content = "v=DMARC1; p=none; rua=mailto:dmarc@bodyofchrist.app; ruf=mailto:dmarc@bodyofchrist.app; sp=reject; adkim=r; aspf=r; pct=10"
      }
    }
  }
}

resource "cloudflare_dns_record" "zoho_mx" {
  for_each = local.boc.mx
  zone_id  = cloudflare_zone.bodyofchrist.id
  name     = each.value.name
  type     = "MX"
  priority = each.value.priority
  content  = each.value.content
  ttl      = 600
}

resource "cloudflare_dns_record" "zoho_txt" {
  for_each = local.boc.txt
  zone_id  = cloudflare_zone.bodyofchrist.id
  name     = each.value.name
  type     = "TXT"
  content  = each.value.content
  ttl      = 600
}
