var LastestBlocksAjax={
    template: `
<tr class="odd">
    <td><a href="/blockDetail/?&amp;height=2130202" title="902bd6ad39047fc61d3183b4469f2635e599022799fd3a613055420166a2f26c"
            target="_BLANK">2130202</a></td>
    <td><a href="/blockDetail/?hash=902bd6ad39047fc61d3183b4469f2635e599022799fd3a613055420166a2f26c" title="902bd6ad39047fc61d3183b4469f2635e599022799fd3a613055420166a2f26c"
            target="_BLANK">902bd6ad39047fc61d3183b4469f2635e599022799fd3a613055420166a2f26c</a></td>
    <td><span title="2019-02-10 15:31:42">15:31:42</span></td>
    <td><span class="badge">Success</span></td>
    <td>0</td>
</tr>
    `,
    blockState:{
        1:{title:"Success",class:" badge-success"},
        2:{title:"Pending",class:" badge-brand"},
        3:{title:"Delivered",class:" badge-metal"},
        4:{title:"Canceled",class:" badge-primary"},
        5:{title:"Info",class:" badge-info"},
        6:{title:"Danger",class:" badge-danger"},
        7:{title:"Warning",class:" badge-warning"}
    },
    reload:function(){
        $.ajax({
            url : "/data/lastestBlocks.data",
            dataType : 'json',
            success : function (d) {
                var data = d.aaData
                var tbody = $("#fleta_blocks tbody")
                tbody.empty()
                for (var i = 0 ; i < data.length ; i++) {
                    var t = $(LastestBlocksAjax.template)
                    t.attr("class", (i%2==0)?"odd":"even")

                    var tds = t.find("td")
                    tds.eq(0).html('<a href="/blockDetail?height='+data[i]["Block Height"]+'" title="'+data[i]["Block Hash"]+'" target="_BLANK">'+data[i]["Block Height"]+'</a>')
                    tds.eq(1).html('<a href="/blockDetail?hash='+data[i]["Block Hash"]+'" title="'+data[i]["Block Hash"]+'" target="_BLANK">'+data[i]["Block Hash"]+'</a>')

                    var texts = data[i].Time.split(" ")
                    if (texts.length > 1) {
                        tds.eq(2).html('<span title="'+data[i].Time+'">'+texts[1]+'</span>')
                    }
                    tds.eq(2).html('<span title="'+data[i].Time+'">'+data[i].Time+'</span>')

                    var status = void 0===LastestBlocksAjax.blockState[data[i].Status]?data[i].Status:'<span class="badge '+LastestBlocksAjax.blockState[data[i].Status].class+'">'+LastestBlocksAjax.blockState[data[i].Status].title+"</span>"
                    tds.eq(3).html(status)
                    tds.eq(4).html(data[i].Txs)
                    tbody.append(t)
                }
            }
        })
    },
    init:function(recursive){
        LastestBlocksAjax.reload()
        if (recursive !== false) {
            setInterval( function () {
                LastestBlocksAjax.reload();
            }, 3000 );
        }
    }
};



