/* eslint-disable */
import { getLocale, experimentalStaticLocale } from '../runtime.js';

/** @typedef {import('../runtime.js').LocalizedString} LocalizedString */

/** @typedef {{}} Common_LoadingInputs */

const en_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Loading...`)
};

const sv_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Laddar...`)
};

const uk_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Завантаження...`)
};

const zh_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`加载中...`)
};

const es_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Cargando...`)
};

const hi_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`लोड हो रहा है...`)
};

const pt_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`A carregar...`)
};

const bn_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`লোড হচ্ছে...`)
};

const ru_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Загрузка...`)
};

const ja_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`読み込み中...`)
};

const vi_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Đang tải...`)
};

const yue_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`載入中...`)
};

const tr_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Yükleniyor...`)
};

const ar_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`جارٍ التحميل...`)
};

const wuu_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`加载当中...`)
};

const mr_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`लोड होत आहे...`)
};

const nb_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Laster...`)
};

const fi_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Ladataan...`)
};

const da_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Indlæser...`)
};

const et_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Laadimine...`)
};

const lv_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Ielāde...`)
};

const lt_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Kraunama...`)
};

const pl_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Ładowanie...`)
};

const de_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Laden...`)
};

const nl_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Laden...`)
};

const fr_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Chargement...`)
};

const it_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Caricamento...`)
};

const hu_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Betöltés...`)
};

const cs_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Načítání...`)
};

const ro_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Se încarcă...`)
};

const el_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Φόρτωση...`)
};

const bg_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Зареждане...`)
};

const hr_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Učitavanje...`)
};

const sr_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Učitavanje...`)
};

const sk_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Načítanie...`)
};

const sl_common_loading = /** @type {(inputs: Common_LoadingInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Nalaganje...`)
};

/**
* | output |
* | --- |
* | "Loading..." |
*
* @param {Common_LoadingInputs} inputs
* @param {{ locale?: "en" | "sv" | "uk" | "zh" | "es" | "hi" | "pt" | "bn" | "ru" | "ja" | "vi" | "yue" | "tr" | "ar" | "wuu" | "mr" | "nb" | "fi" | "da" | "et" | "lv" | "lt" | "pl" | "de" | "nl" | "fr" | "it" | "hu" | "cs" | "ro" | "el" | "bg" | "hr" | "sr" | "sk" | "sl" }} options
* @returns {LocalizedString}
*/
export const common_loading = /** @type {((inputs?: Common_LoadingInputs, options?: { locale?: "en" | "sv" | "uk" | "zh" | "es" | "hi" | "pt" | "bn" | "ru" | "ja" | "vi" | "yue" | "tr" | "ar" | "wuu" | "mr" | "nb" | "fi" | "da" | "et" | "lv" | "lt" | "pl" | "de" | "nl" | "fr" | "it" | "hu" | "cs" | "ro" | "el" | "bg" | "hr" | "sr" | "sk" | "sl" }) => LocalizedString) & import('../runtime.js').MessageMetadata<Common_LoadingInputs, { locale?: "en" | "sv" | "uk" | "zh" | "es" | "hi" | "pt" | "bn" | "ru" | "ja" | "vi" | "yue" | "tr" | "ar" | "wuu" | "mr" | "nb" | "fi" | "da" | "et" | "lv" | "lt" | "pl" | "de" | "nl" | "fr" | "it" | "hu" | "cs" | "ro" | "el" | "bg" | "hr" | "sr" | "sk" | "sl" }, {}>} */ ((inputs = {}, options = {}) => {
	const locale = experimentalStaticLocale ?? options.locale ?? getLocale()
	if (locale === "en") return en_common_loading(inputs)
	if (locale === "sv") return sv_common_loading(inputs)
	if (locale === "uk") return uk_common_loading(inputs)
	if (locale === "zh") return zh_common_loading(inputs)
	if (locale === "es") return es_common_loading(inputs)
	if (locale === "hi") return hi_common_loading(inputs)
	if (locale === "pt") return pt_common_loading(inputs)
	if (locale === "bn") return bn_common_loading(inputs)
	if (locale === "ru") return ru_common_loading(inputs)
	if (locale === "ja") return ja_common_loading(inputs)
	if (locale === "vi") return vi_common_loading(inputs)
	if (locale === "yue") return yue_common_loading(inputs)
	if (locale === "tr") return tr_common_loading(inputs)
	if (locale === "ar") return ar_common_loading(inputs)
	if (locale === "wuu") return wuu_common_loading(inputs)
	if (locale === "mr") return mr_common_loading(inputs)
	if (locale === "nb") return nb_common_loading(inputs)
	if (locale === "fi") return fi_common_loading(inputs)
	if (locale === "da") return da_common_loading(inputs)
	if (locale === "et") return et_common_loading(inputs)
	if (locale === "lv") return lv_common_loading(inputs)
	if (locale === "lt") return lt_common_loading(inputs)
	if (locale === "pl") return pl_common_loading(inputs)
	if (locale === "de") return de_common_loading(inputs)
	if (locale === "nl") return nl_common_loading(inputs)
	if (locale === "fr") return fr_common_loading(inputs)
	if (locale === "it") return it_common_loading(inputs)
	if (locale === "hu") return hu_common_loading(inputs)
	if (locale === "cs") return cs_common_loading(inputs)
	if (locale === "ro") return ro_common_loading(inputs)
	if (locale === "el") return el_common_loading(inputs)
	if (locale === "bg") return bg_common_loading(inputs)
	if (locale === "hr") return hr_common_loading(inputs)
	if (locale === "sr") return sr_common_loading(inputs)
	if (locale === "sk") return sk_common_loading(inputs)
	return sl_common_loading(inputs)
});