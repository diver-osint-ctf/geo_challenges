CTFd.plugin.run((_CTFd) => {
  const $ = _CTFd.lib.$;

  // Définir les traductions pour le plugin geo_challenges
  const translations = {
    en: {
      click_map:
        "Click on the map to place your marker. A blue circle indicates the tolerance zone.",
      submit_location: "Submit Location",
      select_location_first: "Please select a location on the map first.",
      error_submitting: "Error submitting challenge",
      my_submissions: "Submissions",
      submission_status: "Type",
      submission_coordinates: "Submission",
      submission_date: "Date",
      submission_actions: "Actions",
      no_submissions: "You haven't made any submissions yet.",
      zoom: "Zoom",
      loading_submissions: "Loading submissions...",
      error_loading_submissions:
        "Unable to load submissions. Please try again later.",
    },
    fr: {
      click_map:
        "Cliquez sur la carte pour placer votre marqueur. Un cercle bleu indique la zone de tolérance.",
      submit_location: "Soumettre la position",
      select_location_first:
        "Veuillez d'abord sélectionner un emplacement sur la carte.",
      error_submitting: "Erreur lors de la soumission du défi",
      my_submissions: "Soumissions",
      submission_status: "Type",
      submission_coordinates: "Soumission",
      submission_date: "Date",
      submission_actions: "Actions",
      no_submissions: "Vous n'avez pas encore fait de soumissions.",
      zoom: "Zoomer",
      loading_submissions: "Chargement des soumissions...",
      error_loading_submissions:
        "Impossible de charger les soumissions. Veuillez réessayer plus tard.",
    },
    es: {
      click_map:
        "Haga clic en el mapa para colocar su marcador. Un círculo azul indica la zona de tolerancia.",
      submit_location: "Enviar ubicación",
      select_location_first:
        "Por favor, seleccione primero una ubicación en el mapa.",
      error_submitting: "Error al enviar el desafío",
      my_submissions: "Envíos",
      submission_status: "Tipo",
      submission_coordinates: "Envío",
      submission_date: "Fecha",
      submission_actions: "Acciones",
      no_submissions: "Aún no has hecho ningún envío.",
      zoom: "Ampliar",
      loading_submissions: "Cargando envíos...",
      error_loading_submissions:
        "No se pudieron cargar los envíos. Por favor, inténtalo de nuevo más tarde.",
    },
    ja: {
      click_map:
        "地図上をクリックしてマーカーを置いてください。マーカーを置いた際の青い円は許容誤差範囲を示します。",
      submit_location: "Submit Location",
      select_location_first: "地図上で座標を選択してください。",
      error_submitting: "問題の提出中にエラーが発生しました。",
      my_submissions: "Submissions",
      submission_status: "Type",
      submission_coordinates: "Submission",
      submission_date: "Date",
      submission_actions: "Actions",
      no_submissions: "You haven't made any submissions yet.",
      zoom: "Zoom",
      loading_submissions: "Loading submissions...",
      error_loading_submissions:
        "Unable to load submissions. Please try again later.",
    },
  };

  // Charger la langue du CTFd depuis le cookie
  function getCurrentLanguage() {
    // Fonction pour lire un cookie par son nom
    function getCookie(name) {
      const value = `; ${document.cookie}`;
      const parts = value.split(`; ${name}=`);
      if (parts.length === 2) return parts.pop().split(";").shift();
      return null;
    }

    // Essayer de récupérer la langue depuis le cookie
    let ctfdLang = getCookie("language") || "en";

    // Vérifier si la langue est supportée, sinon utiliser l'anglais
    if (!translations[ctfdLang]) {
      ctfdLang = "en";
    }

    return ctfdLang;
  }

  // Fonction de traduction
  window.GeoChallenge = window.GeoChallenge || {};

  window.GeoChallenge.translate = function (key) {
    const lang = getCurrentLanguage();
    return translations[lang][key] || translations["en"][key] || key;
  };

  // Fonction pour appliquer les traductions aux éléments HTML
  window.GeoChallenge.applyTranslations = function () {
    // Traduire les éléments statiques
    $("#geo-submit").text(window.GeoChallenge.translate("submit_location"));
    $(".map-instructions").text(window.GeoChallenge.translate("click_map"));

    // Translations for My Submissions tab
    $(".my-submissions-text").text(
      window.GeoChallenge.translate("my_submissions")
    );
    $(".submission-status-header").text(
      window.GeoChallenge.translate("submission_status")
    );
    $(".submission-coordinates-header").text(
      window.GeoChallenge.translate("submission_coordinates")
    );
    $(".submission-date-header").text(
      window.GeoChallenge.translate("submission_date")
    );
    $(".submission-actions-header").text(
      window.GeoChallenge.translate("submission_actions")
    );
    $(".no-submissions-text").text(
      window.GeoChallenge.translate("no_submissions")
    );
    $(".loading-submissions-text").text(
      window.GeoChallenge.translate("loading_submissions")
    );
    $(".error-loading-submissions-text").text(
      window.GeoChallenge.translate("error_loading_submissions")
    );
  };

  // Charger les traductions une fois que la page est prête
  $(document).on("shown.bs.modal", function (e) {
    // Attendre un peu pour s'assurer que le modal est complètement affiché
    setTimeout(function () {
      // Vérifier si c'est un challenge de type geo
      if ($("#map-solve").length) {
        window.GeoChallenge.applyTranslations();
      }
    }, 100);
  });
});
