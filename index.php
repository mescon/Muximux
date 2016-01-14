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
				$item .= "<li data-content=\"". $keyname . "\" class=\"selected\"><iframe scrolling=\"auto\" src=\"". $section["url"] . "\" style=\"width:100%; height:926px\"></iframe></li>\n";
			} else {
				$item .= "<li data-content=\"". $keyname . "\"><iframe scrolling=\"auto\" src=\"". $section["url"] . "\" style=\"width:100%; height:926px\"></iframe></li>\n";
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
<html lang="en" class="no-js">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<link rel="shortcut icon" type="image/ico" href="favicon.ico" />
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.0/css/bootstrap.min.css" /> <!-- Bootstrap -->
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/font-awesome/4.5.0/css/font-awesome.min.css" /> <!--FontAwesome-->

	<link href='//fonts.googleapis.com/css?family=PT+Sans:400' rel='stylesheet' type='text/css'> <!-- Font -->
	<link rel="stylesheet" href="css/reset.css"> <!-- CSS reset -->
	<link rel="stylesheet" href="css/style.css"> <!-- Resource style -->
	<script src="js/modernizr.js"></script> <!-- Modernizr -->

	<title>MTPHP - Application Management Console</title>
</head>

<body>
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

<script src="js/jquery-2.1.1.js"></script>
<script src="js/main.js"></script> <!-- Resource jQuery -->
</body>
</html>
