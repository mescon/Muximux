var branch = $("#branch-data").attr("data"), commitURL = "https://api.github.com/repos/mescon/Muximux/commits?sha=" + branch, localversion = $("#sha-data").attr("data"), cwd = $("#cwd-data").attr("data"), phpini = $("#phpini-data").attr("data"), gitdir = $("#gitdirectory-data").attr("data"), title = $("#title-data").attr("data"), branch = $("#branch-data").attr("data"), secret = $("#secret").attr("data"), difference = 0, differenceDays, json;
function checkScrolling(b) {
  var c = parseInt(b.children(".cd-tabs-navigation").width()), a = parseInt(b.width());
  b.scrollLeft() >= c - a ? b.parent(".cd-tabs").addClass("is-ended") : b.parent(".cd-tabs").removeClass("is-ended");
}
function resizeIframe(b, c) {
  var a = b && !c ? $(window).height() - 5 : $(window).height() - $(".cd-tabs-navigation").height();
  $("iframe").css({height:a + "px"});
}
function htmlEntities(b) {
  return String(b).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}
function dropDownFixPosition(b, c) {
  var a = b.offset().top + b.outerHeight();
  c.css("top", a + "px");
  c.css("left", $(window).width() - $(".drop-nav").width() - b.offset().left + "px");
}
function settingsEventHandlers() {
  function b(a) {
    setTimeout(function() {
      $(a).remove();
    }, 1E3);
  }
  $("#sortable").sortable();
  $("input[type=radio]").change(function() {
    $("input[type=radio]:checked").not(this).prop("checked", !1);
  });
  $("#showInstructions").click(function() {
    $("#instructionsContainer").slideToggle(1E3);
    '<span class="fa fa-book"></span> Show Guide' == $(this).html() ? $(this).html('<span class="fa fa-book"></span> Hide Guide') : $(this).html('<span class="fa fa-book"></span> Show Guide');
  });
  $("#showChangelog").click(function() {
    $("#changelogContainer").slideToggle(1E3);
    '<span class="fa fa-github"></span> Show Updates' == $(this).html() ? ($(this).html('<span class="fa fa-github"></span> Hide Updates'), viewChangelog()) : $(this).html('<span class="fa fa-github"></span> Show Updates');
  });
  "" != $("#backupContents").text() && ($("#topButtons").append('<a class="btn btn-primary" id="showBackup"><span class="fa fa-book"></span> Show Backup INI</a>'), $("#topButtons").css("width", "425px"), $("#showBackup").click(function() {
    $("#backupiniContainer").slideToggle(1E3);
    '<span class="fa fa-book"></span> Show Backup INI' == $(this).html() ? $(this).html('<span class="fa fa-book"></span> Hide Backup INI') : $(this).html('<span class="fa fa-book"></span> Show Backup INI');
  }));
  $("#removeAll").click(function() {
    if (confirm("Are you sure?")) {
      var a = {}, c = 150;
      $(this).parents("form").children("#sortable").children().reverse().each(function() {
        var d = $(this);
        setTimeout(function() {
          d.effect("drop", a, 150, b(d));
        }, c);
        c += 150;
      });
      $("#addApplication").click();
    }
  });
  $("form").on("click", ".removeButton", function() {
    confirm("Are you sure?") && $($(this).parents(".applicationContainer")).effect("drop", {}, 500, b($(this).parents(".applicationContainer")));
  });
  $("#removeBackup").click(function() {
    var a = $("#secret").data().data;
    $.ajax({async:!0, url:"muximux.php", type:"GET", data:{remove:"backup", secret:a}, success:function(a) {
      "deleted" == a && ($("#backupiniContainer").toggle(1E3), $("#showBackup").remove(), $("#topButtons").css("width", "280px"));
    }});
  });
  $("body").on("click", ".btn-arrow", function(a) {
    a.preventDefault();
    $(this).hasClass("disabled") ? $(this).attr("disabled", "disabled") : $(".btn-arrow").removeAttr("disabled");
  });
  $("#addApplication").click(function() {
    var a = Math.floor(999999 * Math.random() + 1);
    $("#sortable").append('<div class="applicationContainer newApp" id="' + a + 'newApplication"><span class="bars fa fa-bars"></span><div><br><label>Name:</label><input class="appName ' + a + 'newApplication_-_value" name="' + a + 'newApplication_-_name" type="text" value=""></div><div><br><label>URL:</label><input class="' + a + 'newApplication_-_value" name="' + a + 'newApplication_-_url" type="text" value=""></div><div><br><label>Icon:</label><button class="' + a + 'newApplication_-_value iconpicker btn btn-default" name="' + 
    a + 'newApplication_-_icon"  data-iconset="fontawesome" data-icon=""></button></div><div><label>Color:</label><input type="color" class="appsColor ' + a + 'newApplication_-_color" value="#ffffff" name="' + a + 'newApplication_-_color"></div><div><label for="' + a + 'newApplication_-_enabled">Enable:</label><input class="checkbox ' + a + 'newApplication_-_value" id="' + a + 'newApplication_-_enabled" name="' + a + 'newApplication_-_enabled" type="checkbox" checked></div><div><br><label for="' + 
    a + 'newApplication_-_default">Default:</label><input class="radio ' + a + 'newApplication_-_value" id="' + a + 'newApplication_-_default" name="' + a + 'newApplication_-_default" type="radio"></div><div><label for="' + a + 'newApplication_-_landingpage">Enable landing page:</label><input class="checkbox ' + a + 'newApplication_-_value" id="' + a + 'newApplication_-_landingpage" name="' + a + 'newApplication_-_landingpage" type="checkbox"></div><div><label for="' + a + 'newApplication_-_dd">Put in dropdown:</label><input class="checkbox ' + 
    a + 'newApplication_-_value" id="' + a + 'newApplication_-_dd" name="' + a + 'newApplication_-_dd" type="checkbox"></div><button type="button" class="removeButton btn btn-danger btn-xs" value="Remove" id="remove_-_' + a + 'newApplication">Remove<meta class="newAppRand" value="' + a + '"></button><meta class="newAppRand" value="' + a + '"></div></div>');
    initIconPicker("." + a + "newApplication_-_value[name=" + a + "newApplication_-_icon]");
  });
  $("form").on("focusout", ".appName", function() {
    if ("" != $(this).val()) {
      $(this).parents(".applicationContainer").removeClass("newApp");
      var a = $(this).attr("was");
      void 0 == a && (a = $(this).parents(".applicationContainer").children(".newAppRand").attr("value") + "newApplication", $(this).parents("applicationContainer").children(".newAppRand").remove());
      var b = $(this).val().split(" ").join("_");
      $(this).attr("was", b);
      $("." + a + "-value").each(function() {
        var c = $(this).attr("name").split("-");
        $(this).removeAttr("name").prop("name", b + "-" + c[1]).addClass(b + "-value").removeClass(a + "-value");
      });
      $('input[name="' + a + '-icon"]').prop("name", b + "-icon");
      $(this).parents("div.applicationContainer").attr("id", b);
    }
  });
  var c = {url:"muximux.php", type:"post", success:showResponse};
  $("#settingsSubmit").click(function(a) {
    a.preventDefault();
    $(".newApp").remove();
    $(".checkbox,.radio").each(function() {
      if (!$(this).prop("checked")) {
        var a = $(this).attr("name");
        $('<input type="hidden" name="' + a + '" value="false">').appendTo($(this));
      }
    });
    $(".appName").removeAttr("disabled");
    $("form").ajaxSubmit(c);
  });
  $("#tabcolorCheckbox").click(function(a) {
    $(this).prop("checked");
    $(".appsColor").toggle("slide", {direction:"left"}, 200);
    $(".generalColor").toggle("slide", {direction:"left"}, 200);
  });
}
function viewChangelog() {
  $("#changelog").html("");
  sessionStorage.getItem("JSONData") || updateJson();
  json = JSON.parse(sessionStorage.getItem("JSONData"));
  $.getJSON(commitURL, function(b) {
    json = b;
    b = "https://github.com/mescon/Muximux/compare/" + localversion + "..." + json[0].sha;
    difference = 0;
    for (var c in json) {
      json[c].sha == localversion && (difference = c);
    }
    differenceDays = datediff(json[0].commit.author.date.substring(0, 10));
    c = "<strong>up to date!</strong>";
    0 < difference && (c = "<strong>" + difference + " commits behind!</strong>");
    output = "<p>Your install is currently " + c + "<br/>";
    0 < difference && (output += 'The changes from your version to the latest version can be read <a href="' + b + '" target="_blank">here</a>.</p>');
    output += "<p>Updates to your version of <a href='https://github.com/mescon/Muximux/' target='_blank'>Muximux</a> were uploaded to Github " + (0 == differenceDays ? "today" : differenceDays + (1 == differenceDays ? " day ago" : " days ago")) + ".</p>";
    output += "<div class='btn-group' role='group' aria-label='Buttons' id='topButtons'>";
    0 < difference && (output += "<a class='btn btn-primary' id='downloadUpdate'><span class='fa fa-arrow-circle-down'></span> Install Now</a>");
    output += "<a class='btn btn-primary' id='refreshUpdate'><span class='fa fa-rotate-right'></span> Refresh Updates</a></div>";
    if (0 < difference) {
      output += "<p>Or you can manually download <a href='https://github.com/mescon/Muximux/archive/" + branch + ".zip' target='_blank'>the latest zip here.</a></p>";
      output += "<h3>Changelog (" + branch + ")</h3><ul>";
      c = 0;
      do {
        b = json[c].sha.substring(0, 7);
        var a = htmlEntities(json[c].commit.message.substring(0, 550).replace(/$/, "") + "..."), d = json[c].commit.author.date.substring(0, 10);
        output += "<li><pre>" + d + ' <a href="' + json[c].html_url + '">' + b + "</a>:  " + a + "</li></pre>";
        c++;
      } while (c != difference);
      output += "</ul>";
    }
    $("#changelog").html(output);
    $("#downloadUpdate").click(function() {
      downloadUpdate(json[0].sha);
    });
    $("#refreshUpdate").click(function() {
      refreshBranches();
      updateBox(!0);
    });
  });
}
function refreshBranches() {
  $.ajax({async:!0, url:"muximux.php", type:"GET", data:{action:"branches", secret:secret}, success:function(b) {
    viewChangelog();
  }});
}
function initIconPicker(b) {
  $(b).iconpicker({align:"center", arrowClass:"btn-danger", arrowPrevIconClass:"glyphicon glyphicon-chevron-left", arrowNextIconClass:"glyphicon glyphicon-chevron-right", cols:10, footer:!0, header:!0, iconset:"fontawesome", labelHeader:"{0} of {1} pages", labelFooter:"{0} - {1} of {2} icons", placement:"bottom", rows:5, search:!0, searchText:"Search", selectedClass:"btn-success", unselectedClass:""});
}
function showResponse(b, c) {
  1 == b || "success" == c ? location.pathname = location.pathname : alert("Error!!!-" + b);
}
function datediff(b) {
  b = (new Date(b)).getTime();
  var c = (new Date).getTime();
  return Math.round((c - b) / 864E5);
}
function getSecret() {
  $.ajax({async:!0, dataType:"text", url:"muximux.php?get=secret", type:"GET", success:function(b) {
    $("#secret").data({data:b});
  }});
}
function setCookie(b, c, a) {
  var d = new Date;
  d.setTime(d.getTime() + 864E5 * a);
  a = "expires=" + d.toUTCString();
  document.cookie = b + "=" + c + "; " + a;
}
function getCookie(b) {
  b += "=";
  for (var c = document.cookie.split(";"), a = 0;a < c.length;a++) {
    for (var d = c[a];" " == d.charAt(0);) {
      d = d.substring(1);
    }
    if (0 == d.indexOf(b)) {
      return d.substring(b.length, d.length);
    }
  }
  return "";
}
function setTitle(b) {
  $(document).attr("title", b + " - " + $("#maintitle").attr("data"));
}
function updateBox(b) {
  sessionStorage.getItem("JSONData") && !0 !== b || updateJson();
  json = JSON.parse(sessionStorage.getItem("JSONData"));
  b = "https://github.com/mescon/Muximux/compare/" + localversion + "..." + json[0].sha;
  var c = 0, a;
  for (a in json) {
    json[a].sha == localversion && (c = a);
  }
  datediff(json[0].commit.author.date.substring(0, 10));
  c && (clearInterval(void 0), 0 < c && !getCookie("updateDismiss") && ($("#updateContainer").html("<button type='button' id='updateDismiss' class='close pull-right'>&times;</button><span>You are currently <strong>" + c + "</strong> " + (1 < c ? "commits" : "commit") + " behind!<br/>See <a href='" + b + "' target='_blank'>changelog</a> or <div id='downloadModal'><code>click here</code></div> to install now.</span>"), $("#updateContainer").fadeIn("slow"), $("#downloadModal").click(function() {
    downloadUpdate(json[0].sha);
  })), $("#updateDismiss").click(function() {
    $("#updateContainer").fadeOut("slow");
    setCookie("updateDismiss", "true", 1 / 24);
  }));
}
function updateJson() {
  $.getJSON(commitURL, function(b) {
    jsonString = JSON.stringify(b);
    sessionStorage.setItem("JSONData", jsonString, .00694444);
  });
}
function scaleContent(b, c) {
  var a = $(window).width() / c, d = $(window).height() / c - $("nav").height() / c;
  $(".cd-tabs-content").find('li[data-content="' + b + '"]').children("iframe").css({"-ms-transform":"scale(" + c + ")", "-moz-transform":"scale(" + c + ")", "-o-transform":"scale(" + c + ")", "-webkit-transform":"scale(" + c + ")", transform:"scale(" + c + ")", height:"" + d + "px", width:"" + a + "px"});
}
function downloadUpdate(b) {
  confirm("Would you like to download and install updates now?") ? $.ajax({async:!0, url:"muximux.php", type:"GET", data:{action:"update", secret:secret, sha:b}, success:function(b) {
    console.log("DownloadResult: " + b);
    b ? setStatus("Update installed successfully!") : setStatus("An error has occurred.  Please reload and try again.");
  }}) : console.log("Update cancelled.");
}
function write_log(b, c) {
  $.ajax({async:!0, url:"muximux.php", type:"GET", data:{action:"writeLog", secret:secret, msg:b, lvl:c}});
}
function setStatus(b) {
  $("#updateContainer").hide();
  $("#updateContainer").html('<button type="button" id="updateDismiss" class="close pull-right">&times;</button><span>' + b + "<br/><div id='reloadModal'><code>Click to reload</code></div></span>");
  $("#updateContainer").fadeIn("slow");
  $("#reloadModal").click(function() {
    location.reload();
  });
  $("#updateDismiss").click(function() {
    $("#updateContainer").fadeOut("slow");
  });
}
function scaleFrames() {
  $(".cd-tabs-content").find("li").each(function(b) {
    content = $(this).attr("data-content");
    scale = $(this).attr("data-scale");
    $("#" + content + '-scale option[value="' + scale + '"]').attr("selected", "selected");
    "1" !== scale && scaleContent(content, scale);
  });
}
function changeFavicon(b) {
  document.head = document.head || document.getElementsByTagName("head")[0];
  var c = document.createElement("link"), a = document.getElementById("dynamic-favicon");
  c.id = "dynamic-favicon";
  c.rel = "shortcut icon";
  c.href = b;
  a && document.head.removeChild(a);
  document.head.appendChild(c);
}
function htmlDecode(b) {
  return $("<div/>").html(b).text();
}
function getsupportedprop(b) {
  for (var c = document.documentElement, a = 0;a < b.length;a++) {
    if (b[a] in c.style) {
      return b[a];
    }
  }
}
;