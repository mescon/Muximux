$(document).ready(function() {
	$('.wrapper').animate({
		top: "10%"
	}, 300, 'easeInOutElastic', function() {
		$('.logo').delay(1200).slideDown(400, 'easeInOutElastic');
		$('.login0').delay(1000).slideDown(400, 'easeInOutElastic');
		$('.login1').delay(1100).slideDown(400, 'easeInOutElastic');
		$('.login2').delay(1200).slideDown(400, 'easeInOutElastic');
		$('.login3').delay(1300).slideDown(400, 'easeInOutElastic');
		// Animation complete.
	});
});


function setBarColors() {
	var color = rgb2hex($('.logo path').css("fill"));
	$('.droidtheme').replaceWith('<meta name="theme-color" class="droidtheme" content="' + color + '" />');
	$('.mstheme').replaceWith('<meta name="msapplication-navbutton-color" class="mstheme" content="' + color + '" />');
	$('.iostheme').replaceWith('<meta name="apple-mobile-web-app-status-bar-style" class="iostheme" content="' + color + '" />');
}setTimeout(setBarColors, 1650);


function rgb2hex(rgb) {
     if (  rgb.search("rgb") == -1 ) {
          return rgb;
     } else {
          rgb = rgb.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*(\d+))?\)$/);
          function hex(x) {
               return ("0" + parseInt(x).toString(16)).slice(-2);
		}
          return "#" + hex(rgb[1]) + hex(rgb[2]) + hex(rgb[3]); 
		}
}

