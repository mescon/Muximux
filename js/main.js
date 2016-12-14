var boxshadowprop, branch, tabColor, isMobile, overrideMobile, hasDrawer, color, themeColor, tabs, activeTitle;
jQuery(document).ready(function(a) {
  a.extend(a.expr[":"], {containsInsensitive:function(a, c, e, k) {
    return 0 <= (a.textContent || a.innerText || "").toLowerCase().indexOf((e[3] || "").toLowerCase());
  }});
  branch = a("#branch-data").attr("data");
  hasDrawer = "true" == a("#drawer").attr("data");
  tabColor = "true" == a("#tabcolor").attr("data");
  themeColor = a("#themeColor-data").attr("data");
  authentication = a("#authentication-data").attr("data");
  tabs = a(".cd-tabs");
  activeTitle = a("li .selected").attr("data-title");
  getSecret();
  muximuxMobileResize();
  a("#override").css("display", isMobile ? "block" : "none");
  a(".inputdiv").css("display", authentication ? "block" : "none");
  overrideMobile = !1;
  setTitle(activeTitle);
  boxshadowprop = getsupportedprop(["boxShadow", "MozBoxShadow", "WebkitBoxShadow"]);
  a(".drop-nav").toggleClass("hide-nav");
  tabs.each(function() {
    var b = a(this), c = b.find("ul.cd-tabs-navigation, .main-nav"), e = b.children("ul.cd-tabs-content"), b = b.find("nav");
    c.on("click", "a:not(#reload, #hamburger, #override, #logout,#log)", function(b) {
      isMobile || (a(".drop-nav").addClass("hide-nav"), a(".drop-nav").removeClass("show-nav"));
      resizeIframe(hasDrawer, isMobile);
      b.preventDefault();
      b = a(this);
      color = tabColor ? b.attr("data-color") : themeColor;
      if (!b.hasClass("selected")) {
        var f = b.data("content"), d = e.find('li[data-content="' + f + '"]'), f = d.innerHeight();
        b.dblclick(function() {
          d.children("iframe").attr("src", d.children("iframe").attr("src"));
        });
        var g = d.children("iframe").attr("src"), h = d.children("iframe").data("src");
        void 0 !== g && "" !== g || d.children("iframe").attr("src", h);
        "Settings" != b.attr("data-title") && (clearColors(), c.find("a.selected").removeClass("selected"), b.addClass("selected"), setSelectedColor(), b = b.attr("data-title"), setTitle(b), d.addClass("selected").siblings("li").removeClass("selected"), e.animate({height:f}, 200));
      }
    });
    checkScrolling(b);
    b.on("scroll", function() {
      checkScrolling(a(this));
    });
  });
  a("li.dd").on("click", function() {
    toggleClasses();
  });
  a("#reload").on("click", function() {
    a(".fa-refresh").addClass("fa-spin");
    setTimeout(function() {
      a(".fa-refresh").removeClass("fa-spin");
    }, 3900);
    var b = a(".cd-tabs-content").find(".selected").children("iframe");
    b.attr("src", b.attr("src"));
  });
  a("#override").on("click", function() {
    overrideMobile = !overrideMobile;
    muximuxMobileResize();
    overrideMobile && isMobile ? a("#override").addClass("or-active") : a("#override").removeClass("or-active");
  });
  a("#authenticationCheckbox").click(function() {
    a(this).is(":checked") ? a(".inputdiv").slideDown("fast") : a(".inputdiv").slideUp("fast");
  });
  a("#logout").click(function() {
    window.location.href = "?logout";
  });
  a("#settingsModal").on("show.bs.modal", function() {
    setTitle("Settings");
  });
  a("#settingsModal").on("hidden.bs.modal", function() {
    var b = a(".cd-tabs-content").find(".selected").children("iframe").attr("data-title");
    setTitle(b);
  });
  a("#logModal").on("hidden.bs.modal", function() {
    var b = a(".cd-tabs-content").find(".selected").children("iframe").attr("data-title");
    setTitle(b);
  });
  a(window).on("resize", function() {
    tabs.each(function() {
      var b = a(this);
      checkScrolling(b.find("nav"));
      b.find(".cd-tabs-content").css("height", "auto");
    });
    resizeIframe(hasDrawer, isMobile);
    scaleFrames();
  });
  a(".dd").click(function() {
    dropDownFixPosition(a(".dd"), a(".drop-nav"));
  });
  a("#autohideCheckbox").click(function() {
    a("#mobileoverrideCheckbox").prop("checked", !1);
  });
  a("#mobileoverrideCheckbox").click(function() {
    a("#autohideCheckbox").prop("checked", !1);
  });
  a(".dd").mouseleave(function() {
    0 == a(".drop-nav:hover").length && 0 == a(".dd:hover").length && (timeoutId = setTimeout(function() {
      a(".drop-nav").addClass("hide-nav");
      a(".drop-nav").removeClass("show-nav");
    }, 500));
  });
  jQuery.fn.reverse = [].reverse;
  a(".drawerItem").mouseleave(function() {
    a(".drawerItem").removeClass("full");
  });
  a(".drawerItem").mouseenter(function() {
    a(".drawerItem").addClass("full");
  });
  settingsEventHandlers();
  scaleFrames();
  resizeIframe(hasDrawer, isMobile);
  initIconPicker(".iconpicker");
  if (a(location).attr("hash")) {
    var c = a(location).attr("hash").substr(1).replace("%20", " ").replace("_", " ");
    a(document).find('a:containsInsensitive("' + c + '")').trigger("click");
  }
  a("#tabcolorCheckbox").prop("checked") ? (a(".appsColor").show(), a(".generalColor").hide()) : (a(".appsColor").hide(), a(".generalColor").show());
  a("#settingsLogo").click(function() {
    window.open("https://github.com/mescon/Muximux", "_blank");
  });
});
$(window).load(function() {
  "true" == $("#popupdate").attr("data") && setInterval(updateBox(!1), 6E5);
});
$("html").on("keyup", function(a) {
  27 !== a.keyCode || $("#modal-dialog").hasClass("no-display") || $(".close").trigger("click");
});
$(window).unload(function() {
  var a = $("#secret").attr("data");
  $.ajax({async:!0, dataType:"text", url:"muximux.php?secret=" + a + "&set=secret", type:"GET"});
});
$(window).resize(muximuxMobileResize);
function muximuxMobileResize() {
  isMobile = 800 > $(window).width();
  $("#override").css("display", isMobile ? "block" : "none");
  if (isMobile && !overrideMobile) {
    $(".cd-tabs-navigation nav").children().appendTo(".drop-nav");
    var a = .8 * $(window).height();
    $(".drop-nav").css("max-height", a + "px");
  } else {
    $(".drop-nav").children(".cd-tab").appendTo(".cd-tabs-navigation nav");
    $(".drop-nav").css("max-height", "");
    var c = 0;
    $(".cd-tab").each(function() {
      var a = $(this).attr("data-index");
      $(this).width() + c > $(window).width() - $(".main-nav").width() ? $(".drop-nav").insertAt(a, this) : $(".cd-tabs-navigation nav").insertAt(a, this);
      c += $(this).width();
    });
  }
  clearColors();
  setSelectedColor();
}
jQuery.fn.insertAt = function(a, c) {
  var b = this.children().size();
  0 > a && (a = Math.max(0, b + 1 + a));
  this.append(c);
  a < b && this.children().eq(a).before(this.children().last());
  return this;
};
function toggleClasses() {
  $(".drop-nav").toggleClass("hide-nav");
  $(".drop-nav").toggleClass("show-nav");
}
function clearColors() {
  $(".selected").children("span").css("color", "");
  $(".selected").css("color", "");
  $(".selected").css("Box-Shadow", "");
}
function setSelectedColor() {
  color = tabColor ? $("li .selected").attr("data-color") : themeColor;
  $(".droidtheme").replaceWith('<meta name="theme-color" class="droidtheme" content="' + color + '" />');
  $(".mstheme").replaceWith('<meta name="msapplication-navbutton-color" class="mstheme" content="' + color + '" />');
  $(".iostheme").replaceWith('<meta name="apple-mobile-web-app-status-bar-style" class="iostheme" content="' + color + '" />');
  isMobile && !overrideMobile ? ($(".cd-tabs-bar").removeClass("drawer"), $(".cd-tab").removeClass("drawerItem"), $(".navbtn").removeClass("drawerItem"), $(".cd-tabs-bar").removeClass("drawerItem"), $(".selected").children("span").css("color", "" + color + ""), $(".selected").css("color", "" + color + "")) : ($(".selected").css("Box-Shadow", "inset 0 5px 0 " + color + ""), hasDrawer ? ($(".cd-tab").addClass("drawerItem"), $(".navbtn").addClass("drawerItem"), $(".cd-tabs-bar").addClass("drawerItem")) : 
  ($(".cd-tab").removeClass("drawerItem"), $(".navbtn").removeClass("drawerItem"), $(".cd-tabs-bar").removeClass("drawerItem")));
}
;