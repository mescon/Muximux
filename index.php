<!doctype html>
<?php
$config = parse_ini_file('config.ini.php', true);

function menuItems($config) {
	if (empty($item)) $item = '';
	foreach ($config as $keyname => $section) {
		if(!empty($section["enabled"]) && !($section["enabled"]=="false") && ($section["enabled"]=="true")) {
			if(!empty($section["default"]) && !($section["default"]=="false") && ($section["default"]=="true")) {
				$item .= "<li><a data-content=\"" . $keyname . "\" href=\"#0\" class=\"selected\"><span class=\"". $section["icon"] ." fa-lg\"></span> ". $section["name"] ."</a></li>\n";
			} else {
				$item .= "<li><a data-content=\"" . $keyname . "\" href=\"#0\"><span class=\"". $section["icon"] ." fa-lg\"></span> ". $section["name"] ."</a></li>\n";
			}
		}
	}
	if (empty($item)) $item = '';
	return $item;
}


function frameContent($config) {
	if (empty($item)) $item = '';
	foreach ($config as $keyname => $section) {
		if(!empty($section["landingpage"]) && !($section["landingpage"]=="false") && ($section["landingpage"]=="true")) {
			$section["url"] = "?landing=" . $keyname;
		}

		if(!empty($section["enabled"]) && !($section["enabled"]=="false") && ($section["enabled"]=="true")) {
			if(!empty($section["default"]) && !($section["default"]=="false") && ($section["default"]=="true")) {
				$item .= "<li data-content=\"". $keyname . "\" class=\"selected\"><iframe scrolling=\"auto\" src=\"". $section["url"] . "\"></iframe></li>\n";
			} else {
				$item .= "<li data-content=\"". $keyname . "\"><iframe scrolling=\"auto\" src=\"". $section["url"] . "\"></iframe></li>\n";
			}
		}
	}
	return $item;
}


function landingPage($config, $keyname) {
	$item = "
	<html lang=\"en\">
	<head>
	<title>". $config[$keyname]["name"] ."</title>
	<link rel=\"stylesheet\" href=\"css/landing.css\">
	</head>
	<body>
	<div class=\"login\">
		<div class=\"heading\">
			<h2><span class=\"". $config[$keyname]["icon"] ." fa-3x\"></span></h2>
			<section>
	        	<a href=\"". $config[$keyname]["url"] ."\" target=\"_self\" title=\"Launch ". $config[$keyname]["name"] ."!\"><button class=\"float\">Launch ". $config[$keyname]["name"] ."</button></a>
			</section>
		</div>
	 </div>
	 </body></html>";
	if (empty($item)) $item = '';
	return $item;
}

if(isset($_GET['landing'])) {
	$keyname = $_GET['landing'];
	echo landingPage($config, $keyname);
	die();
}
?>
<!--[if lt IE 7]>      <html class="no-js lt-ie9 lt-ie8 lt-ie7" lang="en"> <![endif]-->
<!--[if IE 7]>         <html class="no-js lt-ie9 lt-ie8" lang="en"> <![endif]-->
<!--[if IE 8]>         <html class="no-js lt-ie9" lang="en"> <![endif]-->
<!--[if gt IE 8]><!--> <html class="no-js" lang="en"> <!--<![endif]-->
<head>
	<meta charset="UTF-8">
	<meta http-equiv="X-UA-Compatible" content="IE=edge,chrome=1">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<meta name="description" content="MTPHP - Application Management Console">
	<link rel="shortcut icon" href="favicon.ico" type="image/ico" />
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.0/css/bootstrap.min.css" /> <!-- Bootstrap -->
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/font-awesome/4.5.0/css/font-awesome.min.css" /> <!--FontAwesome-->
	<link rel="stylesheet" href="//fonts.googleapis.com/css?family=PT+Sans:400" type="text/css"> <!-- Font -->
	<link rel="stylesheet" href="css/reset.css"> <!-- CSS reset -->
	<link rel="stylesheet" href="css/style.css"> <!-- Resource style -->
	<script src="js/modernizr-2.8.3-respond-1.4.2.min.js"></script> <!-- Modernizr -->

	<title>MTPHP - Application Management Console</title>
</head>

<body>
<!--[if lt IE 8]>
	<p class="browserupgrade">You are using an <strong>outdated</strong> browser. Please <a href="http://browsehappy.com/">upgrade your browser</a> to improve your experience.</p>
<![endif]-->

<div class="cd-tabs">
	<nav>
		<ul class="cd-tabs-navigation">
		<?php echo menuItems($config); ?>
		</ul>
	</nav>

	<ul class="cd-tabs-content">
		<?php echo frameContent($config); ?>
	</ul>
</div>

<script src="js/jquery-2.2.0.min.js"></script>
<script src="js/main.js"></script> <!-- Resource jQuery -->
</body>
</html>
