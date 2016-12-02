$(document).ready(function() {
		$("#muximux-logo").css("fill", "#F00");
		$('.wrapper').animate({top: "5%"}, 100, 'easeInOutElastic', function() {
            $('.logo').delay(1200).slideDown(400,'easeInOutElastic');
            $('.login0').delay(1000).slideDown(400,'easeInOutElastic');
            $('.login1').delay(1100).slideDown(400,'easeInOutElastic');
            $('.login2').delay(1200).slideDown(400,'easeInOutElastic');
            $('.login3').delay(1300).slideDown(400,'easeInOutElastic');
        // Animation complete.
        });

});

jQuery('img.svg').each(function(){
            var $img = jQuery(this);
            var imgID = $img.attr('id');
            var imgClass = $img.attr('class');
            var imgURL = $img.attr('src');

            jQuery.get(imgURL, function(data) {
                // Get the SVG tag, ignore the rest
                var $svg = jQuery(data).find('svg');

                // Add replaced image's ID to the new SVG
                if(typeof imgID !== 'undefined') {
                    $svg = $svg.attr('id', imgID);
                }
                // Add replaced image's classes to the new SVG
                if(typeof imgClass !== 'undefined') {
                    $svg = $svg.attr('class', imgClass+' replaced-svg');
                }

                // Remove any invalid XML tags as per http://validator.w3.org
                $svg = $svg.removeAttr('xmlns:a');

                // Replace image with new SVG
                $img.replaceWith($svg);

            }, 'xml');

        });
