# permanentdetour
A tiny web service which redirects Sierra Web OPAC requests to Primo URLs. 

```
Permanent Detour: A tiny web service which redirects Sierra Web OPAC requests to Primo URLs.
Usage: permanentdetour [flag...] [file...]
  -address string
        Address to bind on. (default ":8877")
  -primo string
        The subdomain of the target Primo instance, ?????.primo.exlibrisgroup.com. Required.
  -vid string
        VID parameter for Primo. Required.
  Environment variables read when flag is unset:
  PERMANENTDETOUR_ADDRESS
  PERMANENTDETOUR_PRIMO
  PERMANENTDETOUR_VID
```

The following redirects are supported (with examples in the Carleton context):

- Permalinks. `/record=b2405380` is redirected to `https://ocul-crl.primo.exlibrisgroup.com/discovery/fulldisplay?docid=alma991018705459705153&vid=01OCUL_CRL:CRL_DEFAULT`
- Patron login. `/patroninfo` is redirected to `https://ocul-crl.primo.exlibrisgroup.com/discovery/login?vid=01OCUL_CRL:CRL_DEFAULT`
- Author index, call number index, and title search index. `/search/a?SEARCH=twain&sortdropdown=-&searchscope=9` is redirected to `https://ocul-crl.primo.exlibrisgroup.com/discovery/browse?browseQuery=twain&browseScope=author&vid=01OCUL_CRL:CRL_DEFAULT`
- Some "canned" searches. `/search/?searchtype=t&SORT=D&searcharg=spiders&searchscope=9&submit=Submit` redirects to `https://ocul-crl.primo.exlibrisgroup.com/discovery/search?query=title,contains,spiders&search_scope=MyInst_and_CI&tab=Everything&vid=01OCUL_CRL:CRL_DEFAULT`
