$(document).ready(function() {
  $(".wrapper").animate({top:"10%"}, 300, "easeInOutElastic", function() {
    $(".logo").delay(1200).slideDown(400, "easeInOutElastic");
    $(".login0").delay(1E3).slideDown(400, "easeInOutElastic");
    $(".login1").delay(1100).slideDown(400, "easeInOutElastic");
    $(".login2").delay(1200).slideDown(400, "easeInOutElastic");
    $(".login3").delay(1300).slideDown(400, "easeInOutElastic");
  });
});
function setBarColors() {
  var a = rgb2hex($(".logo path").css("fill"));
  $(".droidtheme").replaceWith('<meta name="theme-color" class="droidtheme" content="' + a + '" />');
  $(".mstheme").replaceWith('<meta name="msapplication-navbutton-color" class="mstheme" content="' + a + '" />');
  $(".iostheme").replaceWith('<meta name="apple-mobile-web-app-status-bar-style" class="iostheme" content="' + a + '" />');
}
setTimeout(setBarColors, 1650);
function rgb2hex(a) {
  if (-1 == a.search("rgb")) {
    return a;
  }
  var b = function(a) {
    return ("0" + parseInt(a).toString(16)).slice(-2);
  };
  a = a.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*(\d+))?\)$/);
  return "#" + b(a[1]) + b(a[2]) + b(a[3]);
}
;